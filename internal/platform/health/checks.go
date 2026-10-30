package health

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func SQLPing(db *pgxpool.Pool) Check {
	return func(ctx context.Context) error {
		if db == nil {
			return fmt.Errorf("db is nil")
		}
		ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		return db.Ping(ctx2)
	}
}

// GRPCHealthCheck checks downstream readiness using the standard gRPC health service.
func GRPCHealthCheck(conn *grpc.ClientConn, service string) Check {
	return func(ctx context.Context) error {
		if conn == nil {
			return fmt.Errorf("grpc conn is nil")
		}
		c := healthpb.NewHealthClient(conn)
		ctx2, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		resp, err := c.Check(ctx2, &healthpb.HealthCheckRequest{Service: service})
		if err != nil {
			return err
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("health status: %s", resp.Status.String())
		}
		return nil
	}
}
