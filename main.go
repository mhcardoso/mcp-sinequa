package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	DocSearchBase = "https://docsearch.sinequa.com/"
)

func createQueryURL(query string) string {
	return fmt.Sprintf("%s/app/tech-doc-ns/#/search?query={\"name\":\"tech-doc-pf-ns-en_query\",\"text\":\"%s\"}", DocSearchBase, query)
}

var username string
var password string
var browser *rod.Browser
var debug = false

func createBrowser() (*rod.Browser) {
	if path, exists := launcher.LookPath(); exists {
		u := launcher.New().Headless(!debug).Bin(path).MustLaunch()
		return rod.New().ControlURL(u).MustConnect()
	} else {
		log.Fatal("There was no browser available.")
	}
	return nil
}

func logIntoSinequa(page *rod.Page) {
	page.MustNavigate(DocSearchBase)
	page.MustWaitLoad()
	page.MustWaitStable()
	info := page.MustInfo()
	if !strings.Contains(info.URL, "login.coe") {
		return
	}

	page.MustElement("#username").MustInput(username)
	page.MustElement("#password").MustInput(password).MustType(input.Enter)
	page.MustWaitLoad()
	page.MustWaitStable()
}

func createListOfDocumentURLSForQuery(query string) ([]string, error) {
	page := browser.MustPage()

	logIntoSinequa(page)

	page.MustNavigate(createQueryURL(query))
	page.MustWaitLoad()
	page.MustWaitStable()
	elems := page.MustElements(".sq-result-title")

	neural, err:= page.Element(".text-end")
	if err != nil {
		log.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?P<answers>\d+) answers found in (?P<documents>\d+) documents`)
	neuralText, err := neural.Text()
	if err != nil {
		log.Fatal(err)
	}

	answersAndDocuments := pattern.FindAllStringSubmatch(neuralText, -1)
	numAnswers, err := strconv.Atoi(answersAndDocuments[0][1])
	if err != nil {
		log.Fatal(err)
	}

	numDocuments, err := strconv.Atoi(answersAndDocuments[0][2])
	if err != nil {
		log.Fatal(err)
	}

	documents := []string{}
	for i := range numDocuments {
		elem, err := elems[numAnswers+i].Property("href")
		if err != nil {
			log.Fatal(err)
		}
		documents = append(documents, elem.String())
	}

	return documents, nil
}

func createDocumentationString(urls []string, browser *rod.Browser) (string, error) {
	page := browser.MustPage()
	result := []string{}
	for _, document := range urls {
		page.MustNavigate(document)
		html := page.MustElement("#content").MustHTML()
		markdown, err := htmltomarkdown.ConvertString(html)
		if err != nil {
			return "", err
		}
		result = append(result, markdown)
	}

	return strings.Join(result, "\n"), nil
}

func getDocumentation(ctx context.Context, req *mcp.CallToolRequest, input DocumentationInput) (
	*mcp.CallToolResult, any, error,
) {
	query := input.Query
	listOfURLS, err := createListOfDocumentURLSForQuery(query)
	if err != nil {
		log.Fatal(err)
	}
	result, err := createDocumentationString(listOfURLS, browser)
	if err != nil {
		log.Fatal(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

type DocumentationInput struct {
	Query string `json:"query" jsonschema:"Two or three word query to search for in the Sinequa documentatino"`
}

func init() {
	args := os.Args
	if len(args) == 2 {
		if args[1] == "debug" {
			debug = true
		}
	}
	if env_username, exists := os.LookupEnv("SINEQUA_USERNAME"); exists {
		username = env_username
	} else {
		log.Fatalln("No SINEQUA_USERNAME environment variable")
	}
	if env_password, exists := os.LookupEnv("SINEQUA_PASSWORD"); exists {
		password = env_password
	} else {
		log.Fatalln("No SINEQUA_PASSWORD environment variable")
	}

	browser = createBrowser()
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name: "mcp-sinequa",
		Version: "0.0.1",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_documentation",
		Description: "Returns the html of the top documentation pages related to the query, as defined by Sinequa Neural Search",
	}, getDocumentation)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

