package sa

import (
	"github.com/findy-network/findy-agent/agent/utils"
	"github.com/golang/glog"

	"github.com/findy-network/findy-agent/agent/didcomm"
	"github.com/findy-network/findy-agent/agent/mesg"
	"github.com/findy-network/findy-agent/agent/pltype"
	"github.com/findy-network/findy-wrapper-go/anoncreds"
	"github.com/findy-network/findy-wrapper-go/dto"
)

const (
	PermissiveSA = "permissive_sa"
)

func init() {
	Add(PermissiveSA, permissiveHandler)
}

func permissiveHandler(WDID, plType string, im didcomm.Msg) (om didcomm.Msg, err error) {
	glog.V(1).Info("SA API call received:", plType, im.Info())

	switch plType {
	case pltype.CANotifyStatus:

	case pltype.SAPing:
		om = im
		om.SetReady(true)
		om.SetInfo("SA ping OK")

	case pltype.SAIssueCredentialAcceptPropose:
		om = im
		// in real case, make sure data matches the credential proposal
		om.SetReady(true)

	case pltype.SAPresentProofAcceptPropose:
		om = im
		// todo: this should be get somewhere?
		attrInfo := anoncreds.AttrInfo{
			Name: "email",
		}
		reqAttrs := map[string]anoncreds.AttrInfo{
			"attr1_referent": attrInfo,
		}
		nonce := utils.NewNonceStr()
		proofRequest := anoncreds.ProofRequest{
			Name:                "FirstProofReq",
			Version:             "0.1",
			Nonce:               nonce,
			RequestedAttributes: reqAttrs,
			RequestedPredicates: map[string]anoncreds.PredicateInfo{},
		}
		reqStr := dto.ToJSON(proofRequest)
		om.SetSubMsg(mesg.SubFromJSON(reqStr))
		om.SetReady(true)
	case pltype.SAPresentProofAcceptValues:
		om = im

		// Sample how SA value verification is written
		proofJSON := dto.ToJSON(im.SubMsg())
		var proof anoncreds.Proof
		dto.FromJSONStr(proofJSON, &proof)
		emailToVerify := proof.RequestedProof.RevealedAttrs["attr1_referent"].Raw
		glog.V(1).Info("Testing mock cannot REALLY verify this: ", emailToVerify)

		om.SetReady(true)
	}
	return om, nil
}