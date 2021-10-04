// Package presentproof is Aries protocol processor for present proof protocol.
package presentproof

import (
	"encoding/gob"
	"errors"
	"strconv"

	"github.com/findy-network/findy-agent/agent/comm"
	"github.com/findy-network/findy-agent/agent/didcomm"
	"github.com/findy-network/findy-agent/agent/pltype"
	"github.com/findy-network/findy-agent/agent/prot"
	"github.com/findy-network/findy-agent/agent/psm"
	"github.com/findy-network/findy-agent/agent/utils"
	"github.com/findy-network/findy-agent/protocol/presentproof/prover"
	"github.com/findy-network/findy-agent/protocol/presentproof/task"
	"github.com/findy-network/findy-agent/protocol/presentproof/verifier"
	"github.com/findy-network/findy-agent/std/presentproof"
	pb "github.com/findy-network/findy-common-go/grpc/agency/v1"
	"github.com/findy-network/findy-wrapper-go/anoncreds"
	"github.com/findy-network/findy-wrapper-go/dto"
	"github.com/golang/glog"
	"github.com/lainio/err2"
	"github.com/lainio/err2/assert"
)

type statusPresentProof struct {
	Attributes []didcomm.ProofAttribute `json:"attributes"`
}

var presentProofProcessor = comm.ProtProc{
	Creator:     createPresentProofTask,
	Starter:     startProofProtocol,
	Continuator: continueProtocol,
	Handlers: map[string]comm.HandlerFunc{
		pltype.HandlerPresentProofPropose:      handleProtocol,
		pltype.HandlerPresentProofRequest:      handleProtocol,
		pltype.HandlerPresentProofPresentation: handleProtocol,
		pltype.HandlerPresentProofACK:          handleProtocol,
		pltype.HandlerPresentProofNACK:         handleProtocol,
	},
	Status: getPresentProofStatus,
}

func init() {
	gob.Register(&task.TaskPresentProof{})
	prot.AddCreator(pltype.CAProofPropose, presentProofProcessor)
	prot.AddCreator(pltype.CAProofRequest, presentProofProcessor)
	prot.AddStarter(pltype.CAProofPropose, presentProofProcessor)
	prot.AddStarter(pltype.CAProofRequest, presentProofProcessor)
	prot.AddContinuator(pltype.CAContinuePresentProofProtocol, presentProofProcessor)
	prot.AddStatusProvider(pltype.ProtocolPresentProof, presentProofProcessor)
	comm.Proc.Add(pltype.ProtocolPresentProof, presentProofProcessor)
}

func createPresentProofTask(header *comm.TaskHeader, protocol *pb.Protocol) (t comm.Task, err error) {
	defer err2.Annotate("createIssueCredentialTask", &err)

	proof := protocol.GetPresentProof()
	assert.P.True(proof != nil, "present proof data missing")
	assert.P.True(
		protocol.GetRole() == pb.Protocol_INITIATOR || protocol.GetRole() == pb.Protocol_ADDRESSEE,
		"role is needed for proof protocol")

	// attributes - mandatory
	var proofAttrs []didcomm.ProofAttribute
	if proof.GetAttributesJSON() != "" {
		dto.FromJSONStr(proof.GetAttributesJSON(), &proofAttrs)
		glog.V(3).Infoln("set proof attrs from json:", proof.GetAttributesJSON())
	} else {
		assert.P.True(proof.GetAttributes() != nil, "present proof attributes data missing")
		proofAttrs = make([]didcomm.ProofAttribute, len(proof.GetAttributes().GetAttributes()))
		for i, attribute := range proof.GetAttributes().GetAttributes() {
			proofAttrs[i] = didcomm.ProofAttribute{
				ID:        attribute.ID,
				Name:      attribute.Name,
				CredDefID: attribute.CredDefID,
			}
		}
		glog.V(3).Infoln("set proof from attrs")
	}

	// predicates - optional
	var proofPredicates []didcomm.ProofPredicate
	if proof.GetPredicatesJSON() != "" {
		dto.FromJSONStr(proof.GetPredicatesJSON(), &proofPredicates)
		glog.V(3).Infoln("set proof predicates from json:", proof.GetPredicatesJSON())
	} else if proof.GetPredicates() != nil {
		proofPredicates = make([]didcomm.ProofPredicate, len(proof.GetPredicates().GetPredicates()))
		for i, predicate := range proof.GetPredicates().GetPredicates() {
			proofPredicates[i] = didcomm.ProofPredicate{
				ID:     predicate.ID,
				Name:   predicate.Name,
				PType:  predicate.PType,
				PValue: predicate.PValue,
			}
		}
		glog.V(3).Infoln("set proof from predicates")
	}

	glog.V(1).Infof(
		"Create task for PresentProof with connection id %s, role %s",
		header.ConnID,
		protocol.GetRole().String(),
	)

	return &task.TaskPresentProof{
		TaskBase:        comm.TaskBase{TaskHeader: *header},
		ProofAttrs:      proofAttrs,
		ProofPredicates: proofPredicates,
	}, nil
}

func generateProofRequest(proofTask *task.TaskPresentProof) *anoncreds.ProofRequest {
	reqAttrs := make(map[string]anoncreds.AttrInfo)
	for index, attr := range proofTask.ProofAttrs {
		restrictions := make([]anoncreds.Filter, 0)
		if attr.CredDefID != "" {
			restrictions = append(restrictions, anoncreds.Filter{CredDefID: attr.CredDefID})
		}
		id := "attr_referent_" + strconv.Itoa(index+1)
		if attr.ID != "" {
			id = attr.ID
		}
		reqAttrs[id] = anoncreds.AttrInfo{
			Name:         attr.Name,
			Restrictions: restrictions,
		}
	}
	reqPredicates := make(map[string]anoncreds.PredicateInfo)
	if proofTask.ProofPredicates != nil {
		for index, predicate := range proofTask.ProofPredicates {
			// TODO: restrictions
			id := "predicate_" + strconv.Itoa(index+1)
			if predicate.ID != "" {
				id = predicate.ID
			}
			reqPredicates[id] = anoncreds.PredicateInfo{
				Name:   predicate.Name,
				PType:  predicate.PType,
				PValue: int(predicate.PValue),
			}
		}
	}
	return &anoncreds.ProofRequest{
		Name:                "ProofReq",
		Version:             "0.1",
		Nonce:               utils.NewNonceStr(),
		RequestedAttributes: reqAttrs,
		RequestedPredicates: reqPredicates,
	}
}

func startProofProtocol(ca comm.Receiver, t comm.Task) {
	defer err2.CatchTrace(func(err error) {
		glog.Error(err)
	})

	proofTask, ok := t.(*task.TaskPresentProof)
	assert.P.True(ok)

	switch t.Type() {
	case pltype.CAProofPropose: // ----- prover will start -----
		err2.Check(prot.StartPSM(prot.Initial{
			SendNext:    pltype.PresentProofPropose,
			WaitingNext: pltype.PresentProofRequest,
			Ca:          ca,
			T:           t,
			Setup: func(key psm.StateKey, msg didcomm.MessageHdr) error {
				// Note!! StartPSM() sends certain Task fields to other end
				// as PL.Message like msg.ID, .SubMsg, .Info

				attrs := make([]presentproof.Attribute, len(proofTask.ProofAttrs))
				for i, attr := range proofTask.ProofAttrs {
					attrs[i] = presentproof.Attribute{
						Name:      attr.Name,
						CredDefID: attr.CredDefID,
					}
				}
				pp := presentproof.NewPreviewWithAttributes(attrs)

				propose := msg.FieldObj().(*presentproof.Propose)
				propose.PresentationProposal = pp
				propose.Comment = proofTask.Comment

				rep := &psm.PresentProofRep{
					Key:        key,
					Values:     proofTask.Comment, // TODO: serialize values here?
					WeProposed: true,
				}
				return psm.AddPresentProofRep(rep)
			},
		}))
	case pltype.CAProofRequest: // ----- verifier will start -----
		err2.Check(prot.StartPSM(prot.Initial{
			SendNext:    pltype.PresentProofRequest,
			WaitingNext: pltype.PresentProofPresentation,
			Ca:          ca,
			T:           t,
			Setup: func(key psm.StateKey, msg didcomm.MessageHdr) error {
				// We are started by verifier aka SA, Proof Request comes
				// as startup argument, no need to call SA API to get it.
				// Notice that Proof Req has Nonce, we use the same one for
				// the protocol. UPDATE! After use of Aries message format,
				// we cannot share same Nonce with the proof and messages
				// here. StartPSM() sends certain Task fields to other end
				// as PL.Message
				proofRequest := generateProofRequest(proofTask)
				// get proof req from task came in
				proofReqStr := dto.ToJSON(proofRequest)

				// set proof req to outgoing request message
				req := msg.FieldObj().(*presentproof.Request)
				req.RequestPresentations = presentproof.NewRequestPresentation(
					utils.UUID(), []byte(proofReqStr))

				// create Rep and save it for PSM to run protocol
				rep := &psm.PresentProofRep{
					Key:    key,
					Values: proofTask.Comment, // TODO: serialize attributes here?,
					// Verifier cannot provide this..
					ProofReq: proofReqStr, //  .. but it gives this one.
				}
				return psm.AddPresentProofRep(rep)
			},
		}))
	default:
		glog.Error("unsupported protocol start api type ")
	}
}

func handleProtocol(packet comm.Packet) (err error) {
	var proofTask *task.TaskPresentProof
	if packet.Payload.ThreadID() != "" {
		proofTask = &task.TaskPresentProof{
			TaskBase: comm.TaskBase{TaskHeader: comm.TaskHeader{
				TaskID: packet.Payload.ThreadID(),
				TypeID: packet.Payload.Type(),
			}},
		}
	}

	switch packet.Payload.ProtocolMsg() {
	case pltype.HandlerPresentProofPropose:
		return verifier.HandleProposePresentation(packet, proofTask)
	case pltype.HandlerPresentProofRequest:
		return prover.HandleRequestPresentation(packet, proofTask)
	case pltype.HandlerPresentProofPresentation:
		return verifier.HandlePresentation(packet, proofTask)
	case pltype.HandlerPresentProofACK:
		return handleProofACK(packet, proofTask)
	case pltype.HandlerPresentProofNACK:
		return handleProofNACK(packet, proofTask)
	}
	return errors.New("no present proof handler")
}

func continueProtocol(ca comm.Receiver, im didcomm.Msg) {

	actionType := task.AcceptRequest
	var proofTask *task.TaskPresentProof
	if im.Thread().ID != "" {
		key := &psm.StateKey{
			DID:   ca.WDID(),
			Nonce: im.Thread().ID,
		}
		state, _ := psm.GetPSM(*key) // ignore error for now
		if state != nil {
			proofTask = state.LastState().T.(*task.TaskPresentProof)
			assert.D.True(proofTask != nil, "proof task not found")
			actionType = proofTask.ActionType
		}
	}

	switch actionType {
	case task.AcceptPropose:
		verifier.ContinueProposePresentation(ca, im)
	case task.AcceptValues:
		verifier.ContinueHandlePresentation(ca, im)
	default:
		prover.UserActionProofPresentation(ca, im)
	}

}

// handleProofACK is a protocol handler func at PROVER side.
// Even the inner handler does nothing, the execution of PSM transition
// terminates the state machine which triggers the notification system to the EA
// side.
func handleProofACK(packet comm.Packet, proofTask *task.TaskPresentProof) (err error) {
	return prot.ExecPSM(prot.Transition{
		Packet:      packet,
		SendNext:    pltype.Terminate,
		WaitingNext: pltype.Terminate,
		Task:        proofTask,
		InOut: func(connID string, im, om didcomm.MessageHdr) (ack bool, err error) {
			defer err2.Annotate("proof ACK handler", &err)
			return true, nil
		},
	})
}

// handleProofNACK is a protocol handler func at PROVER (for this version) side.
// Even the inner handler does nothing, the execution of PSM transition
// terminates the state machine which triggers the notification system to the EA
// side.
func handleProofNACK(packet comm.Packet, proofTask *task.TaskPresentProof) (err error) {
	return prot.ExecPSM(prot.Transition{
		Packet:      packet,
		SendNext:    pltype.Terminate,
		WaitingNext: pltype.Terminate,
		Task:        proofTask,
		InOut: func(connID string, im, om didcomm.MessageHdr) (ack bool, err error) {
			defer err2.Annotate("proof NACK handler", &err)
			return false, nil
		},
	})
}

func getPresentProofStatus(workerDID string, taskID string) interface{} {
	defer err2.CatchTrace(func(err error) {
		glog.Error("Failed to set present proof status: ", err)
	})
	key := &psm.StateKey{
		DID:   workerDID,
		Nonce: taskID,
	}

	proofRep, err := psm.GetPresentProofRep(*key)
	err2.Check(err)

	if proofRep != nil {
		return statusPresentProof{
			Attributes: proofRep.Attributes,
		}
	}

	return nil
}
