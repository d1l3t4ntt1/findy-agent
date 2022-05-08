package core

import (
	"github.com/findy-network/findy-agent/agent/managed"
	"github.com/findy-network/findy-agent/agent/service"
	"github.com/findy-network/findy-agent/agent/storage/api"
)

type DID interface {

	//	Resolve() DIDDoc
	// Validate() error

	DOC() DIDDoc
	NewDoc(ae service.Addr) DIDDoc

	KID() string
	Did() string // this is alias for KID() TODO: remove when done with ssi.DID

	StartEndp(storageH managed.Wallet, connectionID string)
	Store(mgdWallet, mgdStorage managed.Wallet)
	SavePairwiseForDID(mStorage managed.Wallet, theirDID DID, pw PairwiseMeta)
	StoreResult() error
	AEndp() (ae service.Addr, err error)
	SetAEndp(ae service.Addr)

	Route() []string // this usefull for new did methods as well

	String() string // Implementation (key, peer,...) specific behaviour
	SignKey() any
	Packager() api.Packager

	// TODO: this is mainly for indy but could be merged with SignKey?
	VerKey() string

	Storage() managed.Wallet

	URI() string // real URI, currently used in did doc

	// Did() == KID() alias for make old code easy to integrate
}

type MyDID interface {
	DID

	// this won't work because wen can be both: receiver and sender
	Pack(d []byte) ([]byte, error)

	// TODO: took out because singature type
	//	Sign(d []byte) crypto.Signature
}

type TheirDID interface {
	DID

	// this won't work because wen can be both: receiver and sender
	Unpack(d []byte) ([]byte, error)

	// these things could work, all everything with cryptos could
	Verify() error
}

type DIDDoc interface {
}

type Method interface {
}

type Pairwise interface {
	ID() string // DID? Could this be a DID?

	TheirDID() TheirDID
	MyDID() MyDID
}

type Resolver interface {
	Resolve(id DID) DIDDoc
}

type Factor interface {
}

type DidComm interface {
	Did() string
}

type Out interface {
	DidComm

	// TODO: these seem to be found from did doc
	VerKey() string
	Route() []string
	Endpoint() string

	//AEndp() (ae service.Addr, error error) // refactor
}

type In interface {
	Out

	// TODO: these seem to be found from did doc
	Wallet() int
}

type Pipe interface {
	Pack(src []byte) (dst []byte, vk string, err error)
	Unpack(src []byte) (dst []byte, vk string, err error)

	// TODO: do we really need this? propably not when we start to use interface,
	// this was for value object (struct)
	IsNull() bool
}

type Destination struct {
}

type PairwiseMeta struct {
	Name  string
	Route []string
}
