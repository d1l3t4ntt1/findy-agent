package bus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapIndex_AgentAddAnswerer(t *testing.T) {
	keyType := AgentKeyType{
		AgentDID: "AgentDID",
		ClientID: "ClientID",
	}
	qch := WantAllAgentAnswers.AgentAddAnswerer(keyType)
	answerCh := WantAllAgentAnswers.AgentSendQuestion(AgentQuestion{
		AgentNotify: AgentNotify{
			AgentKeyType: keyType,
			ID:           "question-id",
			ConnectionID: "ConnectionID",
		},
	})
	q := <-qch
	assert.Equal(t, q.ID, "question-id")
	assert.Equal(t, q.ConnectionID, "ConnectionID")
	assert.Equal(t, q.ClientID, "ClientID")
	assert.Equal(t, q.AgentDID, "AgentDID")
	WantAllAgentAnswers.AgentSendAnswer(AgentAnswer{
		ID:           q.ID,
		AgentKeyType: keyType,
		Info:         "TestInfo",
	})
	a := <-answerCh
	assert.Equal(t, a.ID, q.ID)
	assert.Equal(t, a.ClientID, q.ClientID)
	assert.Equal(t, a.Info, "TestInfo")
	assert.Equal(t, a.AgentDID, q.AgentDID)

	WantAllAgentAnswers.AgentRmAnswerer(keyType)
}