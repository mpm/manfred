// Package session provides session management for GitHub-triggered workflows.
package session

import (
	"fmt"
	"strings"
)

// Phase represents the current state of a session's workflow.
type Phase string

const (
	// PhasePlanning is the initial phase where Claude creates an implementation plan.
	PhasePlanning Phase = "planning"

	// PhaseAwaitingApproval is when the plan has been posted and awaits user approval.
	PhaseAwaitingApproval Phase = "awaiting_approval"

	// PhaseImplementing is when Claude is actively implementing the approved plan.
	PhaseImplementing Phase = "implementing"

	// PhaseInReview is when a PR has been created and is awaiting review.
	PhaseInReview Phase = "in_review"

	// PhaseRevising is when Claude is addressing PR feedback.
	PhaseRevising Phase = "revising"

	// PhaseCompleted is the terminal state after PR is merged.
	PhaseCompleted Phase = "completed"

	// PhaseError indicates the session encountered an unrecoverable error.
	PhaseError Phase = "error"
)

// AllPhases returns all valid phases.
func AllPhases() []Phase {
	return []Phase{
		PhasePlanning,
		PhaseAwaitingApproval,
		PhaseImplementing,
		PhaseInReview,
		PhaseRevising,
		PhaseCompleted,
		PhaseError,
	}
}

// ValidPhases returns phases that represent active work (not terminal states).
func ActivePhases() []Phase {
	return []Phase{
		PhasePlanning,
		PhaseAwaitingApproval,
		PhaseImplementing,
		PhaseInReview,
		PhaseRevising,
	}
}

// IsValid returns true if the phase is a recognized value.
func (p Phase) IsValid() bool {
	switch p {
	case PhasePlanning, PhaseAwaitingApproval, PhaseImplementing,
		PhaseInReview, PhaseRevising, PhaseCompleted, PhaseError:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the phase is a terminal state (completed or error).
func (p Phase) IsTerminal() bool {
	return p == PhaseCompleted || p == PhaseError
}

// IsActive returns true if the phase represents active work.
func (p Phase) IsActive() bool {
	return p.IsValid() && !p.IsTerminal()
}

// String returns the string representation of the phase.
func (p Phase) String() string {
	return string(p)
}

// DisplayName returns a human-readable name for the phase.
func (p Phase) DisplayName() string {
	switch p {
	case PhasePlanning:
		return "Planning"
	case PhaseAwaitingApproval:
		return "Awaiting Approval"
	case PhaseImplementing:
		return "Implementing"
	case PhaseInReview:
		return "In Review"
	case PhaseRevising:
		return "Revising"
	case PhaseCompleted:
		return "Completed"
	case PhaseError:
		return "Error"
	default:
		return string(p)
	}
}

// ParsePhase parses a string into a Phase.
func ParsePhase(s string) (Phase, error) {
	p := Phase(strings.ToLower(strings.TrimSpace(s)))
	if !p.IsValid() {
		return "", fmt.Errorf("invalid phase: %q", s)
	}
	return p, nil
}

// validTransitions defines the allowed state transitions.
// Key is the current phase, value is the list of phases it can transition to.
var validTransitions = map[Phase][]Phase{
	PhasePlanning:         {PhaseAwaitingApproval, PhaseError},
	PhaseAwaitingApproval: {PhasePlanning, PhaseImplementing, PhaseError},
	PhaseImplementing:     {PhaseInReview, PhaseError},
	PhaseInReview:         {PhaseRevising, PhaseCompleted, PhaseError},
	PhaseRevising:         {PhaseInReview, PhaseError},
	PhaseCompleted:        {}, // Terminal - no transitions
	PhaseError:            {PhasePlanning}, // Can retry from error
}

// CanTransitionTo returns true if a transition from the current phase to the target is valid.
func (p Phase) CanTransitionTo(target Phase) bool {
	allowed, ok := validTransitions[p]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// ValidTransitions returns the list of phases the current phase can transition to.
func (p Phase) ValidTransitions() []Phase {
	return validTransitions[p]
}

// TransitionError represents an invalid state transition.
type TransitionError struct {
	From Phase
	To   Phase
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("invalid transition from %s to %s", e.From, e.To)
}

// ValidateTransition checks if a transition is valid and returns an error if not.
func ValidateTransition(from, to Phase) error {
	if !from.CanTransitionTo(to) {
		return &TransitionError{From: from, To: to}
	}
	return nil
}
