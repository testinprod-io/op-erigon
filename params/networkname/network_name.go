package networkname

const (
	MainnetChainName         = "mainnet"
	SepoliaChainName         = "sepolia"
	GoerliChainName          = "goerli"
	DevChainName             = "dev"
	MumbaiChainName          = "mumbai"
	BorMainnetChainName      = "bor-mainnet"
	BorDevnetChainName       = "bor-devnet"
	GnosisChainName          = "gnosis"
	ChiadoChainName          = "chiado"
	OptimismDevnetChainName  = "optimism-devnet"
	OptimismMainnetChainName = "optimism-mainnet"
	OptimismGoerliChainName  = "optimism-goerli"
)

var All = []string{
	MainnetChainName,
	SepoliaChainName,
	GoerliChainName,
	MumbaiChainName,
	BorMainnetChainName,
	BorDevnetChainName,
	GnosisChainName,
	ChiadoChainName,
	OptimismGoerliChainName,
	OptimismDevnetChainName,
}

// OptimismMainnetChainName is excluded due to genesis alloc mismatch:
// core/allocs/optimism_mainnet.json is empty because its size is too big > 300MB
