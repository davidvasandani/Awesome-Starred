package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

	"golang.org/x/oauth2"

	"github.com/google/go-github/v33/github"
)

type StarredRepositories []*github.StarredRepository

func (starred StarredRepositories) Len() int {
	return len(starred)
}

func (starred StarredRepositories) Less(i, j int) bool {
	return starred[i].StarredAt.After(starred[j].StarredAt.Time)
}

func (starred StarredRepositories) Swap(i, j int) {
	starred[i], starred[j] = starred[j], starred[i]
}

func (starred StarredRepositories) WriteAll(writer *bufio.Writer) error {
	for _, v := range starred {
		name := *v.Repository.Name
		url := *v.Repository.HTMLURL

		desc := ""
		if v.Repository.Description != nil {
			desc = *v.Repository.Description
		}

		content := fmt.Sprintf("\n* [%s](%s) - %s", name, url, desc)
		_, err := writer.WriteString(content)
		if err != nil {
			return err
		}
	}

	return nil
}

type StarChannel chan (StarredRepositories)

func (s StarChannel) Listen(starred *StarredRepositories) {
	for {
		*starred = append(*starred, <-s...)
	}
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	ctx, client := newGithubClient(token)

	var starred StarredRepositories
	var starChan = make(StarChannel)

	go starChan.Listen(&starred)

	starList, initialResp, err := getStarsForPage(1, client, ctx)
	starChan <- starList
	maxPages := initialResp.LastPage

	wg := sync.WaitGroup{}
	for i := initialResp.NextPage; i <= maxPages; i++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()

			starList, _, err = getStarsForPage(page, client, ctx)
			if err != nil {
				return
			}

			starChan <- starList
		}(i)
	}
	wg.Wait()

	sort.Sort(StarredRepositories(starred))

	starred.SaveToFile("README.md")
}

func (starred StarredRepositories) SaveToFile(filename string) {
	file, err := os.Create(filename)
	if err != nil {
		log.Panic(err)
	}

	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString("# Awesome automated list of my starred repositories\n"); err != nil {
		log.Panic(err)
	}

	err = starred.WriteAll(writer)
	if err != nil {
		log.Panic(err)
	}

	writer.Flush()
}

func newGithubClient(token string) (context.Context, *github.Client) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return ctx, client
}

func getStarsForPage(page int, client *github.Client, ctx context.Context) ([]*github.StarredRepository, *github.Response, error) {
	opts := &github.ActivityListStarredOptions{
		Sort:      "created",
		Direction: "desc",
	}

	listOptions := github.ListOptions{Page: page, PerPage: 100}

	opts.ListOptions = listOptions
	starList, resp, err := client.Activity.ListStarred(ctx, "", opts)
	if err != nil {
		return nil, nil, err
	}

	return starList, resp, nil
}
