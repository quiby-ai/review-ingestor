# Review Ingestor Service

A Go microservice that consumes "fetch reviews" tasks from Kafka, retrieves App Store reviews per app ID, country list, and date range, normalizes them into a standardized `Review` model, and publishes success or failure events back to Kafka.