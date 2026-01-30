package logic

import (
	"context"
	"testing"
)

func TestGetServerMapRotation_Performance(t *testing.T) {
	mockCH := &MockConn{}

	// ServerTrackingService only needs CH for this method
	svc := NewServerTrackingService(mockCH, nil, nil)

	ctx := context.Background()
	_, err := svc.GetServerMapRotation(ctx, "server1", 30)
	if err != nil {
		t.Fatalf("GetServerMapRotation failed: %v", err)
	}

	// Optimized calculation:
	// 1. Get map stats (Query) -> Returns 2 maps
	// Total Query calls = 1

	expectedCalls := 1
	if mockCH.QueryCalls != expectedCalls {
		t.Errorf("Expected %d Query calls (Optimized), got %d", expectedCalls, mockCH.QueryCalls)
	}
}
