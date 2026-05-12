package trading

import "fmt"

// Transition applies a lifecycle event to an order state.
func Transition(from OrderStatus, event string) (OrderStatus, error) {
	switch from {
	case OrderStatusNew:
		switch event {
		case "accept":
			return OrderStatusAccepted, nil
		case "reject":
			return OrderStatusRejected, nil
		}
	case OrderStatusAccepted, OrderStatusPartiallyFilled:
		switch event {
		case "fill":
			return OrderStatusFilled, nil
		case "partial_fill":
			return OrderStatusPartiallyFilled, nil
		case "cancel":
			return OrderStatusCanceled, nil
		case "reject":
			return OrderStatusRejected, nil
		}
	}
	return "", fmt.Errorf("invalid transition from %s via %s", from, event)
}

// IsOpen reports whether the order can still receive fills.
func IsOpen(status OrderStatus) bool {
	return status == OrderStatusNew || status == OrderStatusAccepted || status == OrderStatusPartiallyFilled
}
