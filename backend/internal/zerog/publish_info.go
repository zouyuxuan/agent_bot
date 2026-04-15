package zerog

type PublishInfo struct {
	Mode          string
	Reference     string
	TxHash        string
	RootHash      string
	ExplorerTxURL string
	IndexerRPC    string
	EvmRPC        string
	FileLocations []string
}
