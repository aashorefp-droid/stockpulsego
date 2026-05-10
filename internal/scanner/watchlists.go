package scanner

var Watchlists = map[string][]string{
	"default": {
		"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA", "AMD", "NFLX", "CRM",
		"ORCL", "ADBE", "INTC", "PYPL", "SQ", "SHOP", "COIN", "UBER", "ABNB", "SNOW",
		"BA", "CAT", "GS", "JPM", "V", "MA", "DIS", "NKE", "SBUX", "MCD",
		"XOM", "CVX", "PFE", "JNJ", "UNH", "MRNA", "LLY", "ABBV", "BMY", "MRK",
		"SPY", "QQQ", "DIA", "XLF", "XLE", "XLK", "ARKK", "SOXX", "SMH", "MRVL",
	},
	"tech": {
		"AAPL", "MSFT", "NVDA", "GOOGL", "META", "AMD", "TSLA", "ORCL", "ADBE", "CRM",
		"INTC", "QCOM", "TXN", "MU", "AMAT", "LRCX", "KLAC", "MRVL", "AVGO", "ARM",
		"PLTR", "SNOW", "DDOG", "ZS", "CRWD", "NET", "MDB", "SMCI", "DELL", "HPE",
	},
	"mega_cap": {
		"AAPL", "MSFT", "NVDA", "GOOGL", "AMZN", "META", "TSLA", "AVGO", "LLY", "JPM",
		"V", "UNH", "XOM", "MA", "JNJ", "PG", "HD", "COST", "MRK", "ABBV",
	},
	"etfs": {
		"XLB", "XLC", "XLE", "XLF", "XLI", "XLK", "XLP", "XLRE", "XLU", "XLV",
		"XLY", "VAW", "VOX", "VDE", "VFH", "VIS", "VGT", "VDC", "VNQ", "VPU",
		"VHT", "VCR", "RTM", "IXE", "KBE", "ITA", "IYW", "KXI", "REZ", "IDU",
		"IXJ", "RXI", "GLD", "SLV", "DBMF", "HFGM", "CTA", "PPI", "UUP", "TLT",
		"JPST", "BIL", "SHY", "SCHD", "VXUS", "VTI", "IEMG", "SPY", "QQQ", "USMV",
		"SPLV", "GLDM", "TIP", "VTIP", "SCHP", "RINF",
	},
	"momentum": {
		"NVDA", "MRVL", "AVGO", "ARM", "PLTR", "CRWD", "DDOG", "NET", "SMCI",
		"TSLA", "META", "AMZN", "GOOGL", "MSFT", "AMD", "SNOW", "ZS", "SHOP", "COIN", "UBER",
	},
	"short_squeeze": {
		"TSLA", "COIN", "RIVN", "LCID", "NKLA", "BBAI", "SOUN", "MSTR", "IONQ",
		"SMCI", "BYND", "SPCE", "RIDE", "WKHS", "FFIE", "MULN", "ASTS", "RKLB",
		"ACHR", "JOBY", "LILM", "CLOV", "WISH", "SKLZ", "DKNG", "PLBY", "CVNA",
		"GME", "AMC", "BBBY", "KOSS", "EXPR",
	},
}
