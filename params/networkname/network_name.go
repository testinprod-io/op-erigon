package networkname

const (
	MainnetChainName         = "mainnet"
	SepoliaChainName         = "sepolia"
	RinkebyChainName         = "rinkeby"
	GoerliChainName          = "goerli"
	DevChainName             = "dev"
	MumbaiChainName          = "mumbai"
	BorMainnetChainName      = "bor-mainnet"
	BorDevnetChainName       = "bor-devnet"
	GnosisChainName          = "gnosis"
	ChiadoChainName          = "chiado"
	OptimismDevnetChainName  = "op-dev"
	OptimismMainnetChainName = "optimism-mainnet"
	OptimismGoerliChainName  = "optimism-goerli"
)

var All = []string{
	MainnetChainName,
	SepoliaChainName,
	RinkebyChainName,
	GoerliChainName,
	MumbaiChainName,
	BorMainnetChainName,
	BorDevnetChainName,
	GnosisChainName,
	ChiadoChainName,
	OptimismGoerliChainName,
	OptimismDevnetChainName,
}
// OptimismMainnetChainName is excluded due to genesis alloc mismatch
