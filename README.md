This repository contains

- A command-line tool for the [Hydra API](https://siftrics.com/docs/hydra.html).
- The official Go client for the Hydra API. GoDoc [here](https://godoc.org/github.com/siftrics/hydra).

## [Command-line Quickstart](#command-line-quickstart)

Download the latest executable from [the releases page](https://github.com/siftrics/hydra/releases).

### Usage

```
./hydra -d my-data-source receipt_1.jpg receipt_2.pdf --prompt-api-key
```

You must specify the data source in question with `-d` or `--data-source-id`.

You must specify your API key with `--prompt-api-key` or `--api-key-file <filename>`. The latter flag expects a text file containing your API key on a single line.

Run `./hydra` with no flags or arguments to display the full usage section and list all optional flags.

_Mac and Linux users may need to run `chmod u+x hydra` on the downloaded executable before it can be executed._

### Getting an API Key

Go to [https://siftrics.com/](https://siftrics.com/), sign up for an account, create a new data source, and, finally, create an API key by clicking the "Create Key" button on the page of the new data source.

## [Go Client Quickstart](#go-client-quickstart)

Here's the [GoDoc page](https://godoc.org/github.com/siftrics/hydra).

### Complete Example

```
import "github.com/siftrics/hydra"

...

filePaths := []string{"file1.png", "file2.png"}

client := hydra.NewClient(apiKey)
filesChan, err := client.Recognize("my-data-source-id", filePaths...)
if err != nil {
    fmt.Fprintf(os.Stderr, "error: %v\n", err)
    os.Exit(1)
}
for {
    recognizedFile, isOpen := <- filesChan
    if !isOpen {
        break
    }
    filePath := filePaths[recognizedFile.FileIndex]
    if recognizedFile.Error != "" {
        fmt.Fprintf(os.Stderr, "Error processing file '%v'\n", filePath)
        continue
    }
    for label, value := range recognizedFile.RecognizedText {
        str, ok := value.(string)
        if ok {
            fmt.Printf("File '%v', Label '%v': '%v'\n", filePath, label, str)
        } else {
            // This label is not a string, so it must be a table.
            table, err := rf.GetTable(label)
            if err != nil {
                fmt.Fprintf(os.Stderr, "error processing field '%v': %v\n", label, err)
                continue
            }
            fmt.Printf("File '%v', Label '%v':\n", filePath, label)
            for rowIndex, row := range table {
                for columnName, columnValue := range row {
                    fmt.Printf("\tRow %v '%v': '%v'\n", rowIndex, columnName, columnValue)
                }
            }
        }
    }
}
```

### Step-by-Step Guide

Import this repository:

```
import "github.com/siftrics/hydra"
```

Create a client (it is up to you to set up the variable `apiKey`):

```
c := hydra.NewClient(apiKey)
```

Specify your data source and recognize text in files:

```
filesChan, err := client.Recognize(myDataSourceId, "file1.png", "file2.jpeg", "file3.pdf")
if err != nil {
    ...
}
for {
    recognizedFile, isOpen := <- filesChan
    if !isOpen {
        break
    }
    if recognizedFile.Error != "" {
        ...
    }
    ...
}
```

The string `myDataSourceId` is displayed on the web page associated to your data source on siftrics.com.

The `Recognize` function accepts a variable number of strings as input:

```
func (c *Client) Recognize(dataSourceId string, filePaths ...string) (<-chan RecognizedFile, error)
```

The results from `filesChan` are this type:

```
type RecognizedFile struct {
	Error               string
	FileIndex           int
	RecognizedText      map[string]interface{}
}
```

`FileIndex` is the index of the file in the `filePaths ...string` argument. You must use it to determine to which file this `RecognizedFile` is associated.

If `Error` is not an empty string, then there was an error processing the file in question. Otherwise, there were no errors processing the file in question.


### Processing Recognized Text

When a user creates a data source, they draw bounding boxes on a document and label each bounding box. Each bounding box could represent one of three things:

1. a string
2. a table
3. an image

Strings and images are both represented by the `string` type in Go. Images are base64-encoded PNG images.

Tables are represented by the `[]map[string]string` type in Go. That is a slice of `map[string]string` objects.

These are the underlying types of each `interface{}` in the `RecognizedText map[string]interface{}` field of a `RecognizedFile` object.

To make it easier for users to cast from `interface{}` to `[]map[string]string` and `string`, two struct methods have been provided:

1. `func (rf *RecognizedFile) Get(field string) (string, error)`
2. `func (rf *RecognizedFile) GetTable(field string) ([]map[string]string, error)`

The "Complete Example" above demonstrates how to use these functions.

See the [GoDoc page](https://godoc.org/github.com/siftrics/hydra) for complete documentation.

## Cost and Capabilities

The cost of the service is $0.01 per page.

The accuracy and capability of the text recognition is comparable to Google Cloud Vision. It supports more than 100 languages and can handle human handwriting.

## Building from Source

```
go get -u github.com/siftrics/hydra/...
```

This will place the executable command-line tool `hydra` in your `$GOBIN` directory.

If that fails (due to environment variables, go tooling, etc.), you can try

```
$ git clone https://github.com/siftrics/hydra
$ cd hydra/cli
$ go build -o hydra main.go
```

Now the `hydra` executable should be in your current working directory.

## Official API Documentation

You can find the official Hydra API documentation [here](https://siftrics.com/docs/hydra.html).
