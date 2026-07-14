package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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
var programLevel = new(slog.LevelVar)
var th *slog.TextHandler
var logger *slog.Logger


func createBrowser() (*rod.Browser) {
	if path, exists := launcher.LookPath(); exists {
		u := launcher.New().Headless(!debug).Bin(path).MustLaunch()
		logger.Debug("Found a browser in", "path", path)
		return rod.New().ControlURL(u).MustConnect().MustIgnoreCertErrors(true)
	}
	logger.Error("There was no browser available")
	os.Exit(1)
	return nil
}

//This function uses the Must variants and does not return an error
//because there shouldn't be any good reason for a working browser to
//not be able to visit DocSearchBase or be redirected to the login page.
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

	elems, err := page.Elements(".sq-result-title")
	if err != nil {
		logger.Debug("The query had no results")
		return []string{"There were **no documents available** for this query, try a different query."}, nil
	}

	neuralExists, neural, err := page.Has(".text-end")

	if !neuralExists {
		documents := []string{}
		for i := range elems {
			if i >= 3 {
				break
			}
			elem, err := elems[i].Property("href")
			if err != nil {
				return nil, err
			}
			documents = append(documents, elem.String())
		}

		return documents, nil
	} else {
		neuralText, err := neural.Text()
		neuralResultsPattern := regexp.MustCompile(`(?P<answers>\d+) answers found in (?P<documents>\d+) documents`)
		answersAndDocuments := neuralResultsPattern.FindAllStringSubmatch(neuralText, -1)
		numAnswers, err := strconv.Atoi(answersAndDocuments[0][1])
		if err != nil {
			return nil, err
		}

		numDocuments, err := strconv.Atoi(answersAndDocuments[0][2])
		if err != nil {
			return nil, err
		}

		documents := []string{}
		for i := range numDocuments {
			if i >= 3 {
				break
			}
			elem, err := elems[numAnswers+i].Property("href")
			if err != nil {
				return nil, err
			}
			documents = append(documents, elem.String())
		}

		return documents, nil
	}
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
		return nil, nil, err
	}
	result, err := createDocumentationString(listOfURLS, browser)
	if err != nil {
		return nil, nil, err
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
	th = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})
	logger = slog.New(th)
	args := os.Args
	if (len(args) == 2 && args[1] == "debug") || (len(args) == 3 && args[2] == "debug") {
		debug = true
		programLevel.Set(slog.LevelDebug)
		logger.Debug("Debug enabled")
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

