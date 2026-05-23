package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLM is a test double for LLMPort
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{
		Content:    m.response,
		TokensUsed: 10,
	}, m.err
}

func TestClassifierAgent_Execute(t *testing.T) {
	tests := []struct {
		name         string
		llmResponse  string
		llmErr       error
		expectedCat  Category
		expectErr    bool
	}{
		{
			name:        "classifies billing query",
			llmResponse: "billing",
			expectedCat: CategoryBilling,
		},
		{
			name:        "classifies technical query",
			llmResponse: "technical",
			expectedCat: CategoryTechnical,
		},
		{
			name:        "classifies general query",
			llmResponse: "general",
			expectedCat: CategoryGeneral,
		},
		{
			name:        "handles uppercase response",
			llmResponse: "BILLING",
			expectedCat: CategoryBilling,
		},
		{
			name:        "handles response with whitespace",
			llmResponse: "  technical  ",
			expectedCat: CategoryTechnical,
		},
		{
			name:        "handles unknown response",
			llmResponse: "gibberish",
			expectedCat: CategoryUnknown,
		},
		{
			name:      "returns error on llm failure",
			llmErr:    assert.AnError,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			llm := &mockLLM{
				response: tt.llmResponse,
				err:      tt.llmErr,
			}
			agent := NewClassifierAgent(llm)
			state := State{
				SessionID: "test-session",
				Query:     "I need help with my invoice",
			}

			// Act
			result, err := agent.Execute(context.Background(), state)

			// Assert
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCat, result.NextState.Category)
		})
	}
}

func TestClassifierAgent_Type(t *testing.T) {
	agent := NewClassifierAgent(&mockLLM{})
	assert.Equal(t, AgentClassifier, agent.Type())
}

func TestClassifierAgent_DoesNotMutateOriginalState(t *testing.T) {
	llm := &mockLLM{response: "billing"}
	agent := NewClassifierAgent(llm)

	original := State{
		SessionID: "test-session",
		Query:     "billing question",
		Category:  CategoryUnknown,
	}

	_, err := agent.Execute(context.Background(), original)
	require.NoError(t, err)

	// original state must never be mutated
	assert.Equal(t, CategoryUnknown, original.Category)
}