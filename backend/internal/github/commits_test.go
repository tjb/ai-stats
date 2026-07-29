package github

import "testing"

func TestIsClaudeAttributed(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "standard Claude Code trailer",
			message: "feat: add pagination\n\nCo-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>",
			want:    true,
		},
		{
			name:    "claude in name only",
			message: "fix: bug\n\nCo-Authored-By: Claude <claude@anthropic.com>",
			want:    true,
		},
		{
			name:    "anthropic.com domain match",
			message: "chore: cleanup\n\nCo-Authored-By: Assistant <assistant@anthropic.com>",
			want:    true,
		},
		{
			name:    "no trailer",
			message: "fix: simple bug fix",
			want:    false,
		},
		{
			name:    "co-author but not Claude",
			message: "feat: pair programmed\n\nCo-Authored-By: Jane Smith <jane@example.com>",
			want:    false,
		},
		{
			name:    "case insensitive match",
			message: "feat: thing\n\nco-authored-by: CLAUDE SONNET <noreply@ANTHROPIC.COM>",
			want:    true,
		},
		{
			name:    "empty message",
			message: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaudeAttributed(tt.message)
			if got != tt.want {
				t.Errorf("isClaudeAttributed(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}
