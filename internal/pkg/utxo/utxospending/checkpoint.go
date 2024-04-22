package utxospending

import "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"

type UTXOSpendingCheckpoint struct {
	// Previous block information
	// This block needs to be recovered when making DOWN migration
	PrevBlock *CheckpointBlock `json:"prev_block"`

	// New block information
	// This block information needs to be used when making UP migration
	NewBlock *CheckpointBlock `json:"new_block"`

	// List of the transaction outputs with its before migrating content
	// This outputs needs to be recovered when you make DOWN migration
	TxsBeforeUpdate map[string][]*utxostore.TransactionOutput `json:"txs_before_update"`

	// List of addressess needs to be dereferenced after UP migration
	DereferencedAddressesTxs map[string][]string `json:"dereferenced_addresses_txs"`

	// List of the new values for the transactions outputs
	// This transactions needs to be added when you make UP migration
	NewTxOutputs map[string][]*utxostore.TransactionOutput `json:"new_tx_outputs"`

	// List of the new addresses references
	// This txs references needs to be added into address index when
	// making UP migration
	NewAddreessReferences map[string][]string `json:"new_address_refs"`
}

type CheckpointBlock struct {
	Height int64  `json:"height"`
	Hash   string `json:"hash"`
}

func NewUTXOSpendingCheckpoint(
	prevBlock *CheckpointBlock,
	newBlock *CheckpointBlock,
	txsBeforeUpdate map[string][]*utxostore.TransactionOutput,
	dereferencedAddressesTxs map[string][]string,
	newTxOutputs map[string][]*utxostore.TransactionOutput,
	newAddreessReferences map[string][]string,
) *UTXOSpendingCheckpoint {
	return &UTXOSpendingCheckpoint{
		PrevBlock:                prevBlock,
		NewBlock:                 newBlock,
		TxsBeforeUpdate:          txsBeforeUpdate,
		DereferencedAddressesTxs: dereferencedAddressesTxs,
		NewTxOutputs:             newTxOutputs,
		NewAddreessReferences:    newAddreessReferences,
	}
}

func (u *UTXOSpendingCheckpoint) GetPreviousBlockHash() string {
	return u.PrevBlock.Hash
}

func (u *UTXOSpendingCheckpoint) GetDereferencedAddressesTxs() map[string][]string {
	return u.DereferencedAddressesTxs
}

func (u *UTXOSpendingCheckpoint) GetTransactionsBeforeUpdate() map[string][]*utxostore.TransactionOutput {
	return u.TxsBeforeUpdate
}

func (u *UTXOSpendingCheckpoint) GetNewTransactionsOutputs() map[string][]*utxostore.TransactionOutput {
	return u.NewTxOutputs
}

func (u *UTXOSpendingCheckpoint) GetNewAddreessReferences() map[string][]string {
	return u.NewAddreessReferences
}

func (u *UTXOSpendingCheckpoint) GetPreviousBlockheight() int64 {
	return u.PrevBlock.Height
}

func (u *UTXOSpendingCheckpoint) GetNextBlockHash() string {
	return u.NewBlock.Hash
}

func (u *UTXOSpendingCheckpoint) GetNewBlockHeight() int64 {
	return u.NewBlock.Height
}
