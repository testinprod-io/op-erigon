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

	OPDevnetChainName  = "op-devnet"
	OPMainnetChainName = "op-mainnet"
	OPGoerliChainName  = "op-goerli"

	LegacyOPDevnetChainName  = "optimism-devnet"
	LegacyOPMainnetChainName = "optimism-mainnet"
	LegacyOPGoerliChainName  = "optimism-goerli"
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
	OPGoerliChainName,
	OPDevnetChainName,
}

func HandleLegacyName(name string) string {
	switch name {
	case LegacyOPDevnetChainName:
		return OPDevnetChainName
	case LegacyOPGoerliChainName:
		return OPGoerliChainName
	case LegacyOPMainnetChainName:
		return OPMainnetChainName
	default:
		return name
	}
}
