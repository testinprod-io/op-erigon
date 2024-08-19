package networkname

const (
	MainnetChainName        = "mainnet"
	HoleskyChainName        = "holesky"
	SepoliaChainName        = "sepolia"
	GoerliChainName         = "goerli"
	DevChainName            = "dev"
	MumbaiChainName         = "mumbai"
	AmoyChainName           = "amoy"
	BorMainnetChainName     = "bor-mainnet"
	BorDevnetChainName      = "bor-devnet"
	GnosisChainName         = "gnosis"
	BorE2ETestChain2ValName = "bor-e2e-test-2Val"
	ChiadoChainName         = "chiado"

	// op stack chains
	OPDevnetChainName    = "op-devnet"
	OPMainnetChainName   = "op-mainnet"
	OpSepoliaChainName   = "op-sepolia"
	BaseMainnetChainName = "base-mainnet"
	BaseSepoliaChainName = "base-sepolia"

	LegacyOPDevnetChainName  = "optimism-devnet"
	LegacyOPMainnetChainName = "optimism-mainnet"
)

var All = []string{
	MainnetChainName,
	HoleskyChainName,
	SepoliaChainName,
	GoerliChainName,
	MumbaiChainName,
	AmoyChainName,
	BorMainnetChainName,
	BorDevnetChainName,
	GnosisChainName,
	ChiadoChainName,
	OPDevnetChainName,
	OPMainnetChainName,
	OpSepoliaChainName,
	BaseMainnetChainName,
	BaseSepoliaChainName,
}

func HandleLegacyName(name string) string {
	switch name {
	case LegacyOPDevnetChainName:
		return OPDevnetChainName
	case LegacyOPMainnetChainName:
		return OPMainnetChainName
	default:
		return name
	}
}
