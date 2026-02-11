package service

import "fmt"

type Status string

const (
	StatusDraft              Status = "Draft"
	StatusBooked             Status = "Booked"
	StatusInProgress         Status = "InProgress"
	StatusWaitingForApproval Status = "WaitingForApproval"
	StatusCompleted          Status = "Completed"
)

func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusDraft, StatusBooked, StatusInProgress, StatusWaitingForApproval, StatusCompleted:
		return Status(s), nil
	default:
		return "", fmt.Errorf("unknown status: %s", s)
	}
}

var allowedTransitions = map[Status]map[Status]bool{
	StatusDraft:              {StatusBooked: true},
	StatusBooked:             {StatusInProgress: true},
	StatusInProgress:         {StatusWaitingForApproval: true},
	StatusWaitingForApproval: {StatusCompleted: true, StatusInProgress: true},
	StatusCompleted:          {}, // only admin override can reopen
}

func CanTransition(from, to Status) bool {
	m, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return m[to]
}


