package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/convert"
	paramtypes "github.com/ethereum/go-ethereum/params/types"
	"github.com/ethereum/go-ethereum/params/types/common"
	"github.com/ethereum/go-ethereum/params/types/goethereum"
	"github.com/ethereum/go-ethereum/params/types/parity"
	"gopkg.in/urfave/cli.v1"
)

/*

formats: [parity|multigeth|geth|~~aleth(TODO)~~]

? If -[i|in] is not passed, then GUESS the proper config by trial and error. Exit 1 if not found.

> echainspec -[i|in] <format> -[o|out] multigeth [--file=<my/file/path.json|<stdin>]
#@1> <JSON>

> echainspec -[i|in] <format> validate [<stdin>|<my/file/path.json]
#> <exitcode=(0|1)>

> echainspec -[i|in] <format> forks [<stdin>|<my/file/path.json]
#> 1150000
#> 1920000
#> 2250000
#> ...

> echainspec -[i|in] <format> ips [<stdin>|<my/file/path.json]
#> eip2 1150000
#> eip7 1150000
#> eip150 2250000
#> eip155 2650000
#> eip161abc 3000000
#> eip161d 3000000
#> eip170 3000000

*/

var gitCommit = "" // Git SHA1 commit hash of the release (set via linker flags)
var gitDate = ""

var (
	chainspecFormatTypes = map[string]common.Configurator{
		"parity": &parity.ParityChainSpec{},
		"multigeth": &paramtypes.Genesis{
			Config: &paramtypes.MultiGethChainConfig{},
		},
		"geth": &paramtypes.Genesis{
			Config: &goethereum.ChainConfig{},
		},
		// TODO
		// "aleth"
		// "retesteth"
	}
)

var chainspecFormats = func() []string {
	names := []string{}
	for k := range chainspecFormatTypes {
		names = append(names, k)
	}
	return names
}()

var defaultChainspecValues = map[string]common.Configurator{
	"classic": params.DefaultClassicGenesisBlock(),
	"kotti":   params.DefaultKottiGenesisBlock(),
	"mordor":  params.DefaultMordorGenesisBlock(),

	"foundation": params.DefaultGenesisBlock(),
	"ropsten":    params.DefaultTestnetGenesisBlock(),
	"rinkeby":    params.DefaultRinkebyGenesisBlock(),
	"goerli":     params.DefaultGoerliGenesisBlock(),

	"social":      params.DefaultSocialGenesisBlock(),
	"ethersocial": params.DefaultEthersocialGenesisBlock(),
	"mix":         params.DefaultMixGenesisBlock(),
}

var defaultChainspecNames = func() []string {
	names := []string{}
	for k := range defaultChainspecValues {
		names = append(names, k)
	}
	return names
}()

var (
	app = cli.NewApp()

	formatInFlag = cli.StringFlag{
		Name:  "inputf",
		Usage: fmt.Sprintf("Input format type [%s]", strings.Join(chainspecFormats, "|")),
		Value: "",
	}
	fileInFlag = cli.StringFlag{
		Name:  "file",
		Usage: "Path to JSON chain configuration file",
	}
	defaultValueFlag = cli.StringFlag{
		Name:  "default",
		Usage: fmt.Sprintf("Use default chainspec values [%s]", strings.Join(defaultChainspecNames, "|")),
	}
	outputFormatFlag = cli.StringFlag{
		Name:  "outputf",
		Usage: fmt.Sprintf("Output client format type for converted configuration file [%s]", strings.Join(chainspecFormats, "|")),
	}
)

var globalChainspecValue common.Configurator

var errInvalidOutputFlag = errors.New("invalid output format type")
var errNoChainspecValue = errors.New("undetermined chainspec value")
var errInvalidDefaultValue = errors.New("no default chainspec found for name given")
var errInvalidChainspecValue = errors.New("could not read given chainspec")
var errEmptyChainspecValue = errors.New("missing chainspec data")

func mustGetChainspecValue(ctx *cli.Context) error {
	if ctx.NArg() >= 1 {
		if strings.HasPrefix(ctx.Args().First(), "ls-") {
			return nil
		}
		if strings.Contains(ctx.Args().First(), "help") {
			return nil
		}
	}
	if ctx.GlobalIsSet(defaultValueFlag.Name) {
		if ctx.GlobalString(defaultValueFlag.Name) == "" {
			return errNoChainspecValue
		}
		v, ok := defaultChainspecValues[ctx.GlobalString(defaultValueFlag.Name)]
		if !ok {
			return fmt.Errorf("error: %v, name: %s", errInvalidDefaultValue, ctx.GlobalString(defaultValueFlag.Name))
		}
		globalChainspecValue = v
		return nil
	}
	data, err := readInputData(ctx)
	if err != nil {
		return err
	}
	configurator, err := unmarshalChainSpec(ctx.GlobalString(formatInFlag.Name), data)
	if err != nil {
		return err
	}
	globalChainspecValue = configurator
	return nil
}

func convertf(ctx *cli.Context) error {
	c, ok := chainspecFormatTypes[ctx.String(outputFormatFlag.Name)]
	if !ok && ctx.String(outputFormatFlag.Name) == "" {
		b, err := jsonMarshalPretty(globalChainspecValue)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	} else if !ok {
		return errInvalidOutputFlag
	}
	err := convert.Convert(globalChainspecValue, c)
	if err != nil {
		return err
	}
	b, err := jsonMarshalPretty(c)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func init() {
	app.Name = "echainspec"
	app.Usage = "A chain specification and configuration tool for EVM clients"
	//app.Description = "A chain specification and configuration tool for EVM clients"
	app.Version = params.VersionWithCommit(gitCommit, gitDate)
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

USAGE:

- Reading and writing chain configurations:

	The default behavior is to act as a configuration reader and writer (and implicit converter). 
	To establish a target configuration to read, you can either 
		1. Pass in a chain configuration externally, or
		2. Use one of the builtin defaults.

	(1.) When reading an external configuration, specify --inputf to define how the provided
	configuration should be interpreted.

	The tool expects to read from standard input (fd 0). Use --file to specify a filepath instead.

	With an optional --outputf flag, the tool will write the established configuration in the desired format.
	If no --outputf is given, the configuration will be printed in its original format.

	Run the following to list available client formats (both for reading and writing):

		{{.Name}} ls-formats

	(2.) Use --default [<chain>] to set the chain configuration value to one of the built in defaults.
	Run the following to list available default configuration values.

		{{.Name}} ls-defaults

- Inspecting chain configurations:

	Additional commands are provided (see COMMANNDS section) to help grok chain configurations.

EXAMPLES:

	Convert an external chain configuration between client formats (from STDIN)
.
		> cat my-parity-spec.json | {{.Name}} --inputf parity --outputf [geth|multigeth]

	Convert an external chain configuration between client formats (from file).

		> {{.Name}} --inputf parity --file my-parity-spec.json --outputf [geth|multigeth]

	Print a default Ethereum Classic network chain configuration in multigeth format:
	
		> {{.Name}} --default classic --outputf multigeth

	Validate a default Kotti network chain configuration for block #3000000:
	
		> {{.Name}} --default kotti validate 3000000

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`
	log.SetFlags(0)
	app.Flags = []cli.Flag{
		formatInFlag,
		fileInFlag,
		defaultValueFlag,
		outputFormatFlag,
	}
	app.Commands = []cli.Command{
		lsDefaultsCommand,
		lsFormatsCommand,
		validateCommand,
		forksCommand,
		ipsCommand,
	}
	app.Before = mustGetChainspecValue
	app.Action = convertf
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
