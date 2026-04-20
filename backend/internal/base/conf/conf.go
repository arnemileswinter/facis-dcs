package conf

import "time"

func TransactionTimeout() time.Duration {
	return 5 * time.Minute
}

func OutboxProcessorTimeOut() time.Duration {
	return 1 * time.Second
}

func EventBusTopic() string {
	return "digital-contracting-service"
}
