// Package client wraps the k8s client. Useful for mocking
package client

import "time"

const k8sTimeout = 60 * time.Second
