package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Prometheus metrics
var (
	messagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_generator_messages_published_total",
			Help: "Total number of messages published to RabbitMQ",
		},
		[]string{"queue"},
	)

	messagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_generator_messages_consumed_total",
			Help: "Total number of messages consumed from RabbitMQ",
		},
		[]string{"queue"},
	)

	dbQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_generator_db_queries_total",
			Help: "Total number of database queries executed",
		},
		[]string{"operation", "table"},
	)

	dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_generator_db_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)

	ordersCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "test_generator_orders_created_total",
			Help: "Total number of orders created",
		},
	)

	eventsProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "test_generator_events_processed_total",
			Help: "Total number of events processed",
		},
	)

	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_generator_active_connections",
			Help: "Number of active connections",
		},
		[]string{"service"},
	)

	errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_generator_errors_total",
			Help: "Total number of errors",
		},
		[]string{"component", "type"},
	)
)

func init() {
	prometheus.MustRegister(messagesPublished)
	prometheus.MustRegister(messagesConsumed)
	prometheus.MustRegister(dbQueriesTotal)
	prometheus.MustRegister(dbQueryDuration)
	prometheus.MustRegister(ordersCreated)
	prometheus.MustRegister(eventsProcessed)
	prometheus.MustRegister(activeConnections)
	prometheus.MustRegister(errorCounter)
}

func main() {
	log.Println("Starting test generator service...")

	// Get configuration from environment
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://admin:admin123@localhost:5672/")
	postgresURL := getEnv("POSTGRES_URL", "postgres://admin:admin123@localhost:5432/testdb?sslmode=disable")
	metricsPort := getEnv("METRICS_PORT", "8081")

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start metrics server
	go startMetricsServer(metricsPort)

	// Connect to PostgreSQL
	db, err := connectPostgres(postgresURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
	} else {
		defer db.Close()
		activeConnections.WithLabelValues("postgres").Set(1)
		go runDatabaseWorkload(ctx, db)
	}

	// Connect to RabbitMQ
	conn, ch, err := connectRabbitMQ(rabbitmqURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
	} else {
		defer conn.Close()
		defer ch.Close()
		activeConnections.WithLabelValues("rabbitmq").Set(1)
		go runRabbitMQWorkload(ctx, ch)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down test generator...")
	cancel()
	time.Sleep(2 * time.Second)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func startMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	log.Printf("Metrics server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Metrics server failed: %v", err)
	}
}

func connectPostgres(url string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	// Retry connection
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", url)
		if err != nil {
			log.Printf("Attempt %d: Failed to open PostgreSQL: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err != nil {
			log.Printf("Attempt %d: Failed to ping PostgreSQL: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Println("Connected to PostgreSQL")
		return db, nil
	}

	return nil, fmt.Errorf("failed to connect to PostgreSQL after retries: %v", err)
}

func connectRabbitMQ(url string) (*amqp.Connection, *amqp.Channel, error) {
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error

	// Retry connection
	for i := 0; i < 30; i++ {
		conn, err = amqp.Dial(url)
		if err != nil {
			log.Printf("Attempt %d: Failed to connect to RabbitMQ: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		ch, err = conn.Channel()
		if err != nil {
			conn.Close()
			log.Printf("Attempt %d: Failed to open RabbitMQ channel: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Declare queues
		queues := []string{"orders", "events", "notifications"}
		for _, q := range queues {
			_, err = ch.QueueDeclare(q, true, false, false, false, nil)
			if err != nil {
				log.Printf("Warning: Failed to declare queue %s: %v", q, err)
			}
		}

		log.Println("Connected to RabbitMQ")
		return conn, ch, nil
	}

	return nil, nil, fmt.Errorf("failed to connect to RabbitMQ after retries: %v", err)
}

func runDatabaseWorkload(ctx context.Context, db *sql.DB) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	products := []string{"Widget A", "Widget B", "Gadget X", "Gadget Y", "Tool Z"}
	customers := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank"}
	statuses := []string{"pending", "processing", "completed", "cancelled"}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Insert a new order
			start := time.Now()
			customer := customers[rand.Intn(len(customers))]
			product := products[rand.Intn(len(products))]
			quantity := rand.Intn(10) + 1
			price := float64(rand.Intn(20000)+1000) / 100.0

			_, err := db.ExecContext(ctx,
				"INSERT INTO orders (customer_name, product, quantity, price) VALUES ($1, $2, $3, $4)",
				customer, product, quantity, price)

			duration := time.Since(start).Seconds()
			dbQueryDuration.WithLabelValues("insert", "orders").Observe(duration)
			dbQueriesTotal.WithLabelValues("insert", "orders").Inc()

			if err != nil {
				errorCounter.WithLabelValues("postgres", "insert").Inc()
				log.Printf("Failed to insert order: %v", err)
			} else {
				ordersCreated.Inc()
			}

			// Update some orders
			start = time.Now()
			newStatus := statuses[rand.Intn(len(statuses))]
			_, err = db.ExecContext(ctx,
				"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = (SELECT id FROM orders WHERE status = 'pending' ORDER BY RANDOM() LIMIT 1)",
				newStatus)

			duration = time.Since(start).Seconds()
			dbQueryDuration.WithLabelValues("update", "orders").Observe(duration)
			dbQueriesTotal.WithLabelValues("update", "orders").Inc()

			if err != nil && err != sql.ErrNoRows {
				errorCounter.WithLabelValues("postgres", "update").Inc()
			}

			// Query orders count
			start = time.Now()
			var count int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders").Scan(&count)

			duration = time.Since(start).Seconds()
			dbQueryDuration.WithLabelValues("select", "orders").Observe(duration)
			dbQueriesTotal.WithLabelValues("select", "orders").Inc()

			if err != nil {
				errorCounter.WithLabelValues("postgres", "select").Inc()
			}

			// Insert an event
			start = time.Now()
			eventTypes := []string{"order_created", "order_updated", "user_login", "payment_processed"}
			eventType := eventTypes[rand.Intn(len(eventTypes))]
			payload := map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"random":    rand.Intn(1000),
			}
			payloadJSON, _ := json.Marshal(payload)

			_, err = db.ExecContext(ctx,
				"INSERT INTO events (event_type, payload) VALUES ($1, $2)",
				eventType, payloadJSON)

			duration = time.Since(start).Seconds()
			dbQueryDuration.WithLabelValues("insert", "events").Observe(duration)
			dbQueriesTotal.WithLabelValues("insert", "events").Inc()

			if err != nil {
				errorCounter.WithLabelValues("postgres", "insert_event").Inc()
			} else {
				eventsProcessed.Inc()
			}
		}
	}
}

func runRabbitMQWorkload(ctx context.Context, ch *amqp.Channel) {
	publishTicker := time.NewTicker(3 * time.Second)
	defer publishTicker.Stop()

	queues := []string{"orders", "events", "notifications"}

	// Start consumers
	for _, q := range queues {
		go consumeMessages(ctx, ch, q)
	}

	// Publish messages
	for {
		select {
		case <-ctx.Done():
			return
		case <-publishTicker.C:
			for _, q := range queues {
				message := map[string]interface{}{
					"queue":     q,
					"timestamp": time.Now().Unix(),
					"data":      fmt.Sprintf("Test message %d", rand.Intn(10000)),
				}
				body, _ := json.Marshal(message)

				err := ch.PublishWithContext(ctx,
					"",    // exchange
					q,     // routing key (queue name)
					false, // mandatory
					false, // immediate
					amqp.Publishing{
						ContentType: "application/json",
						Body:        body,
					})

				if err != nil {
					errorCounter.WithLabelValues("rabbitmq", "publish").Inc()
					log.Printf("Failed to publish to %s: %v", q, err)
				} else {
					messagesPublished.WithLabelValues(q).Inc()
				}
			}
		}
	}
}

func consumeMessages(ctx context.Context, ch *amqp.Channel, queueName string) {
	msgs, err := ch.Consume(
		queueName,
		"",    // consumer tag
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		log.Printf("Failed to register consumer for %s: %v", queueName, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			// Simulate processing time
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			messagesConsumed.WithLabelValues(queueName).Inc()
			_ = msg // Process message
		}
	}
}
