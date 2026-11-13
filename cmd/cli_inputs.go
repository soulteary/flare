package cmd

import (
	"errors"
	"regexp"
	"strconv"

	flags "github.com/spf13/pflag"
)

// tolerantBool is a boolean flag value that reads the same spellings as the
// environment and .env sources. It reports the "bool" type so help output and
// flag lookups behave exactly as they do for a standard boolean flag.
type tolerantBool bool

func (b *tolerantBool) Set(raw string) error {
	value, ok := parseBoolInput(raw)
	if !ok {
		return errors.New("must be a boolean")
	}
	*b = tolerantBool(value)
	return nil
}

func (b *tolerantBool) Type() string     { return "bool" }
func (b *tolerantBool) String() string   { return strconv.FormatBool(bool(*b)) }
func (b *tolerantBool) IsBoolFlag() bool { return true }

// boolVarP registers a boolean runtime switch. Like pflag's own boolean flags,
// the bare flag without a value means true.
func boolVarP(options *flags.FlagSet, target *bool, name, shorthand string, value bool, usage string) {
	*target = value
	options.VarPF((*tolerantBool)(target), name, shorthand, usage).NoOptDefVal = "true"
}

var printableFlagName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// describeFlagError names the rejected flag without repeating its value: the
// result is shown on the not-ready page as well as in the instance log.
func describeFlagError(err error) string {
	var unknown *flags.NotExistError
	var missing *flags.ValueRequiredError
	var invalid *flags.InvalidValueError
	switch {
	case errors.As(err, &unknown):
		name := unknown.GetSpecifiedName()
		if !printableFlagName.MatchString(name) {
			break
		}
		if unknown.GetSpecifiedShortnames() != "" {
			return "unknown command-line flag -" + name
		}
		return "unknown command-line flag --" + name
	case errors.As(err, &missing):
		return "command-line flag --" + missing.GetFlag().Name + " needs a value"
	case errors.As(err, &invalid):
		message := "invalid value for command-line flag --" + invalid.GetFlag().Name
		if _, tolerant := invalid.GetFlag().Value.(*tolerantBool); tolerant {
			message += "; booleans accept " + acceptedBooleans
		}
		return message
	}
	return "invalid command-line configuration"
}
