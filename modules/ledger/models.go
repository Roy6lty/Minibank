package ledger

type EntryType string

const (
	EntryTypeCredit EntryType = "CREDIT"
	EntryTypeDebit  EntryType = "DEBIT"
)
