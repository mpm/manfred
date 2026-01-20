package session

import (
	"testing"
)

func TestPhaseIsValid(t *testing.T) {
	tests := []struct {
		phase Phase
		want  bool
	}{
		{PhasePlanning, true},
		{PhaseAwaitingApproval, true},
		{PhaseImplementing, true},
		{PhaseInReview, true},
		{PhaseRevising, true},
		{PhaseCompleted, true},
		{PhaseError, true},
		{Phase("invalid"), false},
		{Phase(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.IsValid(); got != tt.want {
				t.Errorf("Phase(%q).IsValid() = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPhaseIsTerminal(t *testing.T) {
	tests := []struct {
		phase Phase
		want  bool
	}{
		{PhasePlanning, false},
		{PhaseAwaitingApproval, false},
		{PhaseImplementing, false},
		{PhaseInReview, false},
		{PhaseRevising, false},
		{PhaseCompleted, true},
		{PhaseError, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.IsTerminal(); got != tt.want {
				t.Errorf("Phase(%q).IsTerminal() = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPhaseCanTransitionTo(t *testing.T) {
	tests := []struct {
		from  Phase
		to    Phase
		valid bool
	}{
		// From Planning
		{PhasePlanning, PhaseAwaitingApproval, true},
		{PhasePlanning, PhaseError, true},
		{PhasePlanning, PhaseImplementing, false},
		{PhasePlanning, PhaseCompleted, false},

		// From Awaiting Approval
		{PhaseAwaitingApproval, PhasePlanning, true},
		{PhaseAwaitingApproval, PhaseImplementing, true},
		{PhaseAwaitingApproval, PhaseError, true},
		{PhaseAwaitingApproval, PhaseCompleted, false},

		// From Implementing
		{PhaseImplementing, PhaseInReview, true},
		{PhaseImplementing, PhaseError, true},
		{PhaseImplementing, PhaseCompleted, false},
		{PhaseImplementing, PhasePlanning, false},

		// From In Review
		{PhaseInReview, PhaseRevising, true},
		{PhaseInReview, PhaseCompleted, true},
		{PhaseInReview, PhaseError, true},
		{PhaseInReview, PhasePlanning, false},

		// From Revising
		{PhaseRevising, PhaseInReview, true},
		{PhaseRevising, PhaseError, true},
		{PhaseRevising, PhaseCompleted, false},

		// From Completed (terminal)
		{PhaseCompleted, PhasePlanning, false},
		{PhaseCompleted, PhaseError, false},

		// From Error (can retry)
		{PhaseError, PhasePlanning, true},
		{PhaseError, PhaseImplementing, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + "->" + string(tt.to)
		t.Run(name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.valid {
				t.Errorf("Phase(%q).CanTransitionTo(%q) = %v, want %v",
					tt.from, tt.to, got, tt.valid)
			}
		})
	}
}

func TestParsePhase(t *testing.T) {
	tests := []struct {
		input   string
		want    Phase
		wantErr bool
	}{
		{"planning", PhasePlanning, false},
		{"PLANNING", PhasePlanning, false},
		{"  planning  ", PhasePlanning, false},
		{"awaiting_approval", PhaseAwaitingApproval, false},
		{"implementing", PhaseImplementing, false},
		{"in_review", PhaseInReview, false},
		{"revising", PhaseRevising, false},
		{"completed", PhaseCompleted, false},
		{"error", PhaseError, false},
		{"invalid", Phase(""), true},
		{"", Phase(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParsePhase(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePhase(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePhase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateTransition(t *testing.T) {
	// Valid transition
	err := ValidateTransition(PhasePlanning, PhaseAwaitingApproval)
	if err != nil {
		t.Errorf("ValidateTransition(planning, awaiting_approval) = %v, want nil", err)
	}

	// Invalid transition
	err = ValidateTransition(PhasePlanning, PhaseCompleted)
	if err == nil {
		t.Error("ValidateTransition(planning, completed) = nil, want error")
	}

	// Check error type
	transErr, ok := err.(*TransitionError)
	if !ok {
		t.Errorf("error is not TransitionError: %T", err)
	} else {
		if transErr.From != PhasePlanning || transErr.To != PhaseCompleted {
			t.Errorf("TransitionError = {%v, %v}, want {planning, completed}",
				transErr.From, transErr.To)
		}
	}
}

func TestPhaseDisplayName(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhasePlanning, "Planning"},
		{PhaseAwaitingApproval, "Awaiting Approval"},
		{PhaseImplementing, "Implementing"},
		{PhaseInReview, "In Review"},
		{PhaseRevising, "Revising"},
		{PhaseCompleted, "Completed"},
		{PhaseError, "Error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.DisplayName(); got != tt.want {
				t.Errorf("Phase(%q).DisplayName() = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}
