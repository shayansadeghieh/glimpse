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
	Number         int
	Author         string // Just the login name
	AuthorID       int64  // The GitHub user ID
	State          string
	CommentThreads map[int64]*CommentThread
}

type CommentThread struct {
	RootComment CodeComment
	Replies     []*CodeComment
	// Sort replies chronologically
	// ResponseTimes []time.Duration // Time between successive replies
}

type CodeComment struct {
	Commentor string
	CommentID int64
	InReplyTo int64
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func findRootComment(comment *CodeComment, commentsByID map[int64]*CodeComment) int64 {
	current := comment

	// Keep following the reply chain until we find a root comment
	for current.InReplyTo != 0 && commentsByID[current.InReplyTo] != nil {
		current = commentsByID[current.InReplyTo]
	}

	// Return the ID of the root comment we found
	return current.CommentID
}

func organizeThreads(comments []CodeComment) map[int64]*CommentThread {
	threads := make(map[int64]*CommentThread)
	commentsByID := make(map[int64]*CodeComment)

	for i := range comments {
		commentsByID[comments[i].CommentID] = &comments[i]
	}

	for i := range comments {
		comment := &comments[i]

		if comment.InReplyTo == 0 {
			threads[comment.CommentID] = &CommentThread{
				RootComment: *comment,
				Replies:     []*CodeComment{},
			}
		} else {

			rootID := findRootComment(comment, commentsByID)
			if thread, exists := threads[rootID]; exists {
				thread.Replies = append(thread.Replies, comment)
			}

		}
	}

	return threads
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
			Number:         *pr.Number,
			Author:         *pr.User.Login,
			AuthorID:       *pr.User.ID,
			State:          *pr.State,
			CommentThreads: make(map[int64]*CommentThread),
		}

		codeComments, _, err := githubClient.PullRequests.ListComments(ctx, config.GithubOwner, config.GithubRepo, *pr.Number, nil)
		if err != nil {
			fmt.Printf("Error listing code comments for PR %d: %v", *pr.Number, err)
			continue
		}

		// Create a slice to hold all comments for this PR
		allComments := make([]CodeComment, 0, len(codeComments))

		for _, comment := range codeComments {
			// In the scenario that the comment is the first comment, there will be no value associated with inReplyTo.
			// As a result, we set the the first comment to be 0
			var inReplyTo int64
			if comment.InReplyTo != nil {
				inReplyTo = *comment.InReplyTo
			}
			allComments = append(allComments, CodeComment{
				Commentor: *comment.User.Login,
				CommentID: *comment.ID,
				InReplyTo: inReplyTo,
				Body:      *comment.Body,
				CreatedAt: *comment.CreatedAt,
				UpdatedAt: *comment.UpdatedAt,
			})

			// Organize comments into threads
			prInfoMap[*pr.ID].CommentThreads = organizeThreads(allComments)
		}
	}

	if pr, exists := prInfoMap[2383247533]; exists {
		fmt.Printf("%+v\n", *pr)

		// Print thread details
		fmt.Println("\nComment Threads:")
		for rootID, thread := range pr.CommentThreads {
			fmt.Printf("Thread %d (Root: %s): %d replies\n",
				rootID, thread.RootComment.Commentor, len(thread.Replies))

			for i, reply := range thread.Replies {
				fmt.Printf("  Reply %d: %s at %s\n",
					i+1, reply.Commentor, reply.CreatedAt.Format(time.RFC3339))
			}
		}
	}

	return nil
}

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
