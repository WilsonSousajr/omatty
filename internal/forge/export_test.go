package forge

// NewCLIWithBin runs bin in place of gh, so a test can stand a script in.
func NewCLIWithBin(bin string) *CLI { return &CLI{bin: bin} }
