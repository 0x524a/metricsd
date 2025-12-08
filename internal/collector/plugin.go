package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	pluginSourceCommand = "command"
	pluginSourceHTTP    = "http"
	pluginSourceFile    = "file"

	pluginParserNumber = "number"
	pluginParserRegex  = "regex"

	defaultPluginTimeout = 10 * time.Second
)

// PluginDefinition describes a custom metric plugin loaded from disk.
type PluginDefinition struct {
	Name            string            `json:"name"`
	Metric          string            `json:"metric"`
	MetricType      string            `json:"metric_type"` // "gauge" or "counter"
	IntervalSeconds int               `json:"interval_seconds"`
	Labels          map[string]string `json:"labels,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds,omitempty"`
	Parser          PluginParser      `json:"parser,omitempty"`
	Command         *CommandSource    `json:"command,omitempty"`
	HTTP            *HTTPSource       `json:"http,omitempty"`
	File            *FileSource       `json:"file,omitempty"`
}

// PluginParser defines how raw plugin output is converted into a numeric value.
type PluginParser struct {
	Mode  string `json:"mode"`            // "number" or "regex"
	Regex string `json:"regex,omitempty"` // required when Mode == "regex"
}

// CommandSource executes a command and parses stdout.
type CommandSource struct {
	// Command to run, split by arguments. Example: ["sh","-c","cat /proc/net/dev"]
	Command []string `json:"command"`
}

// HTTPSource performs an HTTP GET and parses the response body.
type HTTPSource struct {
	URL string `json:"url"`
}

// FileSource reads a file and parses its contents.
type FileSource struct {
	Path string `json:"path"`
}

type pluginRuntime struct {
	def       PluginDefinition
	nextRun   time.Time
	interval  time.Duration
	timeout   time.Duration
	parser    parsedParser
	source    string
	metricKey string
}

type parsedParser struct {
	mode     string
	regex    *regexp.Regexp
	rawRegex string
	rawMode  string
}

// LoadPlugins walks a directory for *.json plugin definition files.
// Each file may contain a single JSON object or an array of objects.
func LoadPlugins(dir string) ([]PluginDefinition, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Info().Str("plugins_dir", dir).Msg("Plugins directory not found; skipping plugin loading")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat plugins dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugins path %s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins dir: %w", err)
	}

	all := make([]PluginDefinition, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read plugin file %s: %w", path, err)
		}

		// Detect object vs array
		raw = bytesTrimSpace(raw)
		if len(raw) == 0 {
			continue
		}

		if raw[0] == '[' {
			var defs []PluginDefinition
			if err := json.Unmarshal(raw, &defs); err != nil {
				return nil, fmt.Errorf("failed to parse plugin file %s: %w", path, err)
			}
			all = append(all, defs...)
			continue
		}

		var def PluginDefinition
		if err := json.Unmarshal(raw, &def); err != nil {
			return nil, fmt.Errorf("failed to parse plugin file %s: %w", path, err)
		}
		all = append(all, def)
	}

	return all, nil
}

// NewPluginCollector creates a collector over validated plugin definitions.
func NewPluginCollector(prefix, pluginsDir string, defs []PluginDefinition) (*PluginCollector, error) {
	runtime := make([]pluginRuntime, 0, len(defs))
	for _, def := range defs {
		prt, err := buildRuntime(def, prefix)
		if err != nil {
			return nil, err
		}
		runtime = append(runtime, prt)
	}

	pc := &PluginCollector{
		prefix:     prefix,
		pluginsDir: pluginsDir,
		plugins:    runtime,
		lastMod:    latestPluginsMTime(pluginsDir),
	}

	return pc, nil
}

// PluginCollector executes user-defined plugins and emits metrics.
type PluginCollector struct {
	prefix     string
	plugins    []pluginRuntime
	pluginsDir string
	lastMod    time.Time
}

// Name returns the collector name.
func (p *PluginCollector) Name() string {
	return "plugin"
}

// Collect executes due plugins respecting per-plugin intervals.
func (p *PluginCollector) Collect(ctx context.Context) ([]Metric, error) {
	p.maybeReload()

	now := time.Now()
	metrics := make([]Metric, 0)

	for i := range p.plugins {
		pl := &p.plugins[i]
		if now.Before(pl.nextRun) {
			continue
		}

		value, err := p.execute(ctx, pl)
		if err != nil {
			log.Warn().Err(err).Str("plugin", pl.def.Name).Msg("Plugin execution failed")
			pl.nextRun = now.Add(pl.interval)
			continue
		}

		labels := make(map[string]string, len(pl.def.Labels)+1)
		for k, v := range pl.def.Labels {
			labels[k] = v
		}
		labels["plugin"] = pl.def.Name

		metrics = append(metrics, Metric{
			Name:   pl.metricKey,
			Labels: labels,
			Value:  value,
			Type:   pl.def.MetricType,
		})

		pl.nextRun = now.Add(pl.interval)
	}

	return metrics, nil
}

// maybeReload hot-reloads plugin definitions when files change.
func (p *PluginCollector) maybeReload() {
	if p.pluginsDir == "" {
		return
	}

	modTime := latestPluginsMTime(p.pluginsDir)
	if !modTime.After(p.lastMod) {
		return
	}

	defs, err := LoadPlugins(p.pluginsDir)
	if err != nil {
		log.Warn().Err(err).Str("plugins_dir", p.pluginsDir).Msg("Failed to reload plugins; keeping previous set")
		return
	}
	runtime := make([]pluginRuntime, 0, len(defs))
	for _, def := range defs {
		prt, err := buildRuntime(def, p.prefix)
		if err != nil {
			log.Warn().Err(err).Str("plugin", def.Name).Msg("Invalid plugin during reload; skipping")
			continue
		}
		runtime = append(runtime, prt)
	}
	p.plugins = runtime
	p.lastMod = modTime

	log.Info().
		Int("plugin_count", len(runtime)).
		Str("plugins_dir", p.pluginsDir).
		Msg("Plugins reloaded from disk")
}

// execute runs a single plugin according to its source.
func (p *PluginCollector) execute(ctx context.Context, pl *pluginRuntime) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, pl.timeout)
	defer cancel()

	switch pl.source {
	case pluginSourceCommand:
		return runCommand(ctx, pl.def.Command, pl.parser)
	case pluginSourceHTTP:
		return runHTTP(ctx, pl.def.HTTP, pl.parser)
	case pluginSourceFile:
		return runFile(ctx, pl.def.File, pl.parser)
	default:
		return 0, fmt.Errorf("unknown plugin source: %s", pl.source)
	}
}

func buildRuntime(def PluginDefinition, prefix string) (pluginRuntime, error) {
	if err := validatePluginDefinition(def); err != nil {
		return pluginRuntime{}, err
	}

	pr, err := normalizeParser(def.Parser)
	if err != nil {
		return pluginRuntime{}, fmt.Errorf("plugin %s: %w", def.Name, err)
	}

	timeout := defaultPluginTimeout
	if def.TimeoutSeconds > 0 {
		timeout = time.Duration(def.TimeoutSeconds) * time.Second
	}

	rt := pluginRuntime{
		def:      def,
		nextRun:  time.Now(),
		interval: time.Duration(def.IntervalSeconds) * time.Second,
		timeout:  timeout,
		parser:   pr,
		source:   detectSource(def),
	}

	rt.metricKey = prefix + def.Metric
	return rt, nil
}

func detectSource(def PluginDefinition) string {
	switch {
	case def.Command != nil:
		return pluginSourceCommand
	case def.HTTP != nil:
		return pluginSourceHTTP
	case def.File != nil:
		return pluginSourceFile
	default:
		return ""
	}
}

func normalizeParser(p PluginParser) (parsedParser, error) {
	mode := p.Mode
	if mode == "" {
		mode = pluginParserNumber
	}

	switch mode {
	case pluginParserNumber:
		return parsedParser{mode: pluginParserNumber, rawMode: mode}, nil
	case pluginParserRegex:
		if p.Regex == "" {
			return parsedParser{}, errors.New("parser regex mode requires 'regex'")
		}
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			return parsedParser{}, fmt.Errorf("invalid parser regex: %w", err)
		}
		return parsedParser{mode: pluginParserRegex, regex: re, rawRegex: p.Regex, rawMode: mode}, nil
	default:
		return parsedParser{}, fmt.Errorf("unsupported parser mode: %s", mode)
	}
}

func validatePluginDefinition(def PluginDefinition) error {
	if def.Name == "" {
		return errors.New("plugin name is required")
	}
	if def.Metric == "" {
		return fmt.Errorf("plugin %s: metric is required", def.Name)
	}
	metricRegex := regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	if !metricRegex.MatchString(def.Metric) {
		return fmt.Errorf("plugin %s: metric '%s' must match regex %s", def.Name, def.Metric, metricRegex.String())
	}

	if def.MetricType != "gauge" && def.MetricType != "counter" {
		return fmt.Errorf("plugin %s: metric_type must be 'gauge' or 'counter'", def.Name)
	}

	if def.IntervalSeconds <= 0 {
		return fmt.Errorf("plugin %s: interval_seconds must be positive", def.Name)
	}

	sourceCount := 0
	if def.Command != nil {
		sourceCount++
		if len(def.Command.Command) == 0 {
			return fmt.Errorf("plugin %s: command source requires 'command' array", def.Name)
		}
		if blacklistedCommand(def.Command.Command[0]) {
			return fmt.Errorf("plugin %s: command '%s' is blacklisted", def.Name, def.Command.Command[0])
		}
	}
	if def.HTTP != nil {
		sourceCount++
		if def.HTTP.URL == "" {
			return fmt.Errorf("plugin %s: http source requires 'url'", def.Name)
		}
	}
	if def.File != nil {
		sourceCount++
		if def.File.Path == "" {
			return fmt.Errorf("plugin %s: file source requires 'path'", def.Name)
		}
	}
	if sourceCount == 0 {
		return fmt.Errorf("plugin %s: one source must be provided (command/http/file)", def.Name)
	}
	if sourceCount > 1 {
		return fmt.Errorf("plugin %s: only one source type is allowed", def.Name)
	}

	return nil
}

func runCommand(ctx context.Context, src *CommandSource, parser parsedParser) (float64, error) {
	if src == nil || len(src.Command) == 0 {
		return 0, errors.New("command source missing")
	}

	cmd := execCommandContext(ctx, src.Command[0], src.Command[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("command failed: %w", err)
	}

	return parseValue(string(output), parser)
}

func runHTTP(ctx context.Context, src *HTTPSource, parser parsedParser) (float64, error) {
	if src == nil || src.URL == "" {
		return 0, errors.New("http source missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Accept", "text/plain, application/json")

	timeout := defaultPluginTimeout
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
	}
	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read http response: %w", err)
	}

	return parseValue(string(body), parser)
}

func runFile(ctx context.Context, src *FileSource, parser parsedParser) (float64, error) {
	if src == nil || src.Path == "" {
		return 0, errors.New("file source missing")
	}

	// Basic context check before reading to honor cancellations quickly.
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	content, err := os.ReadFile(src.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	return parseValue(string(content), parser)
}

func parseValue(raw string, parser parsedParser) (float64, error) {
	switch parser.mode {
	case pluginParserNumber:
		text := strings.TrimSpace(raw)
		return strconv.ParseFloat(text, 64)
	case pluginParserRegex:
		matches := parser.regex.FindStringSubmatch(raw)
		if len(matches) < 2 {
			return 0, fmt.Errorf("regex '%s' did not match any capture group", parser.rawRegex)
		}
		return strconv.ParseFloat(matches[1], 64)
	default:
		return 0, fmt.Errorf("unsupported parser mode %s", parser.rawMode)
	}
}

func blacklistedCommand(cmd string) bool {
	blacklist := map[string]struct{}{
		"rm":       {},
		"sudo":     {},
		"mv":       {},
		"cp":       {},
		"shutdown": {},
		"reboot":   {},
		"halt":     {},
		"init":     {},
	}
	clean := strings.ToLower(filepath.Base(cmd))
	_, blocked := blacklist[clean]
	return blocked
}

func latestPluginsMTime(dir string) time.Time {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}
	}
	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// bytesTrimSpace avoids importing bytes just for TrimSpace.
func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && isSpace(b[start]) {
		start++
	}
	end := len(b)
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r'
}

// execCommandContext is a thin wrapper around exec.CommandContext.
// Separated for testability.
var execCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}
