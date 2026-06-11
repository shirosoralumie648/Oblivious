package task

import (
	"context"
	"log"

	taskpb "oblivious/server/internal/grpc/taskv1"
	"oblivious/server/internal/task/scheduler"
)

type Server struct {
	taskpb.UnimplementedTaskServiceServer
	scheduler *scheduler.CronScheduler
	logger    *log.Logger
}

func NewServer(s *scheduler.CronScheduler, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		scheduler: s,
		logger:    logger,
	}
}

func (s *Server) Schedule(ctx context.Context, req *taskpb.ScheduleRequest) (*taskpb.ScheduleResponse, error) {
	if req.TaskId == "" {
		return &taskpb.ScheduleResponse{Success: false, Message: "task_id is required"}, nil
	}
	if req.CronExpr == "" {
		return &taskpb.ScheduleResponse{Success: false, Message: "cron_expr is required"}, nil
	}

	handler := func(ctx context.Context, entry scheduler.ScheduledEntry) error {
		s.logger.Printf("executing task: %s, payload: %d bytes", entry.TaskID, len(req.Payload))
		return nil
	}

	_, err := s.scheduler.Add(req.TaskId, req.CronExpr, handler)
	if err != nil {
		return &taskpb.ScheduleResponse{Success: false, Message: err.Error()}, nil
	}

	return &taskpb.ScheduleResponse{Success: true, Message: "task scheduled"}, nil
}

func (s *Server) Cancel(ctx context.Context, req *taskpb.CancelRequest) (*taskpb.CancelResponse, error) {
	if req.TaskId == "" {
		return &taskpb.CancelResponse{Success: false, Message: "task_id is required"}, nil
	}

	removed := s.scheduler.Remove(req.TaskId)
	if !removed {
		return &taskpb.CancelResponse{Success: false, Message: "task not found"}, nil
	}

	return &taskpb.CancelResponse{Success: true, Message: "task cancelled"}, nil
}
