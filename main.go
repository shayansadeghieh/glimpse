package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Config struct {
	GithubOwner string
	GithubRepo  string
	GithubUser  string
	Host        string
	Port        string
}

type PRInfo struct {
	Number       int
	Author       string // Just the login name
	AuthorID     int64  // The GitHub user ID
	State        string
	Comments     []GeneralPRComment
	CodeComments []CodeComment
}

type GeneralPRComment struct {
	Commentor string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TODO: Can we combine GeneralPRComment and CodeComment?
type CodeComment struct {
	Commentor string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func run(ctx context.Context) error {
	config := Config{
		GithubOwner: os.Getenv("GITHUB_REPO_OWNER"),
		GithubRepo:  os.Getenv("GITHUB_REPO"),
		Host:        "localhost",
		Port:        "9000",
	}

	pat := os.Getenv("PERSONAL_ACCESS_TOKEN")
	if pat == "" {
		return fmt.Errorf("PERSONAL_ACCESS_TOKEN must be defined")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: pat},
	)

	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	prs, _, err := githubClient.PullRequests.List(ctx, config.GithubOwner, config.GithubRepo, nil)
	if err != nil {
		return fmt.Errorf("Error listing pull requests: %w", err)
	}

	prInfoMap := make(map[int64]*PRInfo)

	for _, pr := range prs {

		prInfoMap[*pr.ID] = &PRInfo{
			Number:       *pr.Number,
			Author:       *pr.User.Login,
			AuthorID:     *pr.User.ID,
			State:        *pr.State,
			Comments:     []GeneralPRComment{},
			CodeComments: []CodeComment{},
		}

		codeComments, _, err := githubClient.PullRequests.ListComments(ctx, config.GithubOwner, config.GithubRepo, *pr.Number, nil)
		if err != nil {
			fmt.Printf("Error listing code comments for PR %d: %v", *pr.Number, err)
			continue
		}

		for _, comment := range codeComments {
			prInfoMap[*pr.ID].CodeComments = append(prInfoMap[*pr.ID].CodeComments, CodeComment{
				Commentor: *comment.User.Login,
				Body:      *comment.Body,
				CreatedAt: *comment.CreatedAt,
				UpdatedAt: *comment.UpdatedAt,
			})
		}

		generalPRComments, _, err := githubClient.Issues.ListComments(ctx, config.GithubOwner, config.GithubRepo, *pr.Number, nil)

		if err != nil {
			fmt.Printf("Error listing general PR comments for PR %d: %v", *pr.Number, err)
			continue
		}

		for _, comment := range generalPRComments {
			prInfoMap[*pr.ID].Comments = append(prInfoMap[*pr.ID].Comments, GeneralPRComment{
				Commentor: *comment.User.Login,
				Body:      *comment.Body,
				CreatedAt: *comment.CreatedAt,
				UpdatedAt: *comment.UpdatedAt,
			})
		}
	}

	fmt.Printf("General PR Comments: %+v\n", *prInfoMap[2383247533])

	return nil
}

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
