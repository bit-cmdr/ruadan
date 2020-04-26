# ruadan

Environment Variable configuration with CLI flag override. Based on the amazing work of Kelsey Hightower's [envconfig](https://github.com/kelseyhightower/envconfig) this aims to solve a similar problem with the addition of cli override flags for each part of your config. Currently you can configure everything with the tags of `envconfig`, `envcli`, `json`, and `clidesc`.

Note: Complex types are not supported yet and only basic, top level, arrays are set up

## API Documentation

See [godoc](https://pkg.go.dev/github.com/bit-cmdr/ruadan?tab=doc)

## Getting Started

```sh
$ go get github.com/bit-cmdr/ruadan
```

### Example Usage

#### Predefined struct

```go
package main

import (
	"log"
	"os"

	rd "github.com/bit-cmdr/ruadan"
)

type config struct {
	TestString string  `envconfig:"TEST_STRING"`
	TestInt    int     `envconfig:"TEST_INT" envcli:"testint"`
	TestFloat  float64 `envconfig:"TEST_FLOAT" envcli:"testfloat" clidesc:"set a float 64 value"`
	Pass       bool    `envcli:"pass"`
}

func main() {
	var cfg config

	fs, err := rd.GetConfigFlagSet(os.Args[1:], &cfg)
	if err != nil {
		log.Fatalf("Unable to configure:\n%v\n", err)
	}

	if !cfg.Pass {
		fs.PrintDefaults()
	}

	log.Printf("read so far:\n%+v\n", cfg)
}
```

#### Undefined struct method

```go
package main

import (
	"log"
	"os"

	rd "github.com/bit-cmdr/ruadan"
)

func main() {
	cfg := rd.BuildConfig(
		rd.NewOption(
      "TestString", 
      rd.OptionENVName("TEST_STRING"), 
      rd.OptionStringDefault(""), 
      rd.OptionCLIName("TEST_STRING"), 
      rd.OptionJSONName("testString"),
    ),
		rd.NewOption(
      "TestInt", 
      rd.OptionENVName("TEST_INT"), 
      rd.OptionInt64Default(0), 
      rd.OptionCLIName("testint"), 
      rd.OptionJSONName("testInt"),
    ),
		rd.NewOption(
      "TestFloat", 
      rd.OptionENVName("TEST_FLOAT"), 
      rd.OptionFloat64Default(0), 
      rd.OptionCLIName("testfloat"), 
      rd.OptionJSONName("testFloat"), 
      rd.OptionCLIUsage("set a float 64 value"),
    ),
		rd.NewOption(
      "Pass", 
      rd.OptionENVName("PASS"), 
      rd.OptionBoolDefault(false), 
      rd.OptionCLIName("pass"), 
      rd.OptionJSONName("pass"),
    ),
  )
  
  // Note that the cfg.Config returned here is already a pointer, there's no need to pass by address
	fs, err := ruadan.GetConfigFlagSet(os.Args[1:], cfg.Config)
	if err != nil {
		log.Fatalf("Unable to configure:\n%v\n", err)
	}

	if !cfg.GetBool("Pass") {
		fs.PrintDefaults()
	}

	log.Printf("read so far:\n%+v\n", cfg)
}
```

#### Output for either method

```sh
$ go run main.go -pass
read so far:
{TestString: TestInt:0 TestFloat:0 Pass:true}

$ go run main.go -testint 1
  -TEST_STRING string
    	flag: TEST_STRING or env: TEST_STRING
  -pass
    	flag: pass or env: PASS
  -testfloat float
    	set a float 64 value
  -testint int
    	flag: testint or env: TEST_INT

read so far:
{TestString: TestInt:1 TestFloat:0 Pass:false}

$ go run main.go -TEST_STRING test
  -TEST_STRING string
    	flag: TEST_STRING or env: TEST_STRING
  -pass
    	flag: pass or env: PASS
  -testfloat float
    	set a float 64 value
  -testint int
    	flag: testint or env: TEST_INT
        
read so far:
{TestString:test TestInt:0 TestFloat:0 Pass:false}

$ PASS=true go run main.go -testint 5 -TEST_STRING testit -testfloat 3.14
read so far:
{TestString:testit TestInt:5 TestFloat:3.14 Pass:true}
```

#### Struct and Tags

```go
type example struct {
    NoTags       int
    EnvConfig    int `envconfig:"EX_CONF"`
    EnvCliConfig int `envcli:"conf"`
    CliDesc      int `clidesc:"simple usage explanation"`
}
```

* `NoTags` will look for an env of `NOTAGS` and a cli of `N0TAGS` and have a description of `flag: NOTAGS or env: NOTAGS`
* `EnvConfig` will look for an env of `EX_CONF` and a cli of `EX_CONF` and have a description of `flag: EX_CONF or env: EX_CONF`
* `EnvCliConfig` will look for an env of `CONF` and a cli of `conf` and have a description of `flag: conf or env: CONF`
* `CliDesc` will look for an env of `CLIDESC` and a cli of `CliDesc` and have a description of `simple usage explanation`

It's meant to be as conventional as possible with the option to be incredibly specific

#### Build Config

```go
cfg := rd.BuildConfig(
  rd.NewOption(
    "Example",
    rd.OptionENVName("ENV_NAME")
    rd.OptionJSONName("jsonName")
    rd.OptionCLIName("cliflagname")
    rd.OptionCLIUsage("use this to describe how to use it from the cli")
    rd.OptionBoolDefault(false)
  )
)
```

* `NewOption` will add a new field to your struct. The first parameter is the name of the field, remember Go's naming conventions for exposing a field and capitalize the first letter. The rest of the Option fields are optional except a default value; That's required to determine type.
* `OptionENVName` is used to set the `envconfig` tag on the field
* `OptionJSONName` is used to set the `json` tag on the field
* `OptionCLIName` is used to set the `envcli` tag on the field
* `OptionCLIUsage` is used to set the `clidesc` tag on the field
* `OptionXDefault` where `X` is the desired type, is used to set the default value and determin the fields final type