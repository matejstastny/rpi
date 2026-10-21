package main

import (
	"fmt"
	"sort"
	"strings"
)

var kindLabels = map[string]string{"anime": "ANIME", "series": "SERIES", "movie": "MOVIES"}

var statusHeadings = []struct {
	status  string
	heading string
}{
	{statusWatching, "CURRENTLY WATCHING"},
	{statusQueue, "QUEUE"},
	{statusBacklog, "IF NOTHING IN QUEUE"},
	{statusSkipped, "NOT WATCHING"},
}

// exportMarkdown is what the copy button puts on the clipboard: the whole
// library in one paste, with the standing instructions on top so a
// recommendation request has everything it needs in a single message
func exportMarkdown(st state, kind string) string {
	var b strings.Builder

	b.WriteString("# WATCHLIST\n\n")

	if notes := strings.TrimSpace(st.Settings.ClaudeInstructions); notes != "" {
		b.WriteString("## Instructions for Claude\n\n")
		b.WriteString(notes)
		b.WriteString("\n\n")
	}

	b.WriteString("## Tier definitions\n\n")
	b.WriteString("These matter. The tiers are not standard S-to-D tiers, read them before ranking anything.\n\n")
	for _, t := range tiers {
		def := strings.TrimSpace(st.Settings.TierDefinitions[t])
		if def == "" {
			continue
		}
		fmt.Fprintf(&b, "- **%s**: %s\n", t, def)
	}
	b.WriteString("\n")

	wanted := kinds
	if kind != "" && kind != "all" {
		wanted = []string{kind}
	}

	for _, k := range wanted {
		pool := []*entry{}
		for _, e := range st.Entries {
			if e.Kind == k {
				pool = append(pool, e)
			}
		}
		if len(pool) == 0 {
			continue
		}

		fmt.Fprintf(&b, "## %s\n\n", kindLabels[k])

		if ranked := byStatus(pool, statusRanked); len(ranked) > 0 {
			b.WriteString("### Ranked\n\n")
			b.WriteString("| TIER | TITLE | FINISHED |\n|:----:|-------|---------:|\n")
			for _, t := range tiers {
				in := []*entry{}
				for _, e := range ranked {
					if e.Tier == t {
						in = append(in, e)
					}
				}
				sortBucket(in)
				for _, e := range in {
					finished := e.Finished
					if finished == "" {
						finished = "NA"
					}
					fmt.Fprintf(&b, "| %s | %s | %s |\n", t, titleLine(e), finished)
				}
			}
			b.WriteString("\n")
		}

		for _, sh := range statusHeadings {
			in := byStatus(pool, sh.status)
			if len(in) == 0 {
				continue
			}
			sortBucket(in)
			fmt.Fprintf(&b, "### %s\n\n", strings.ToUpper(sh.heading[:1])+strings.ToLower(sh.heading[1:]))
			for _, e := range in {
				line := "- " + titleLine(e)
				bits := []string{}
				if e.Progress != "" {
					bits = append(bits, e.Progress)
				}
				if e.Predicted != "" {
					bits = append(bits, "predicting "+e.Predicted)
				}
				if e.Note != "" {
					bits = append(bits, e.Note)
				}
				if len(bits) > 0 {
					line += " — " + strings.Join(bits, ", ")
				}
				b.WriteString(line + "\n")
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func titleLine(e *entry) string {
	out := e.Title
	extras := []string{}
	if e.Year > 0 {
		extras = append(extras, fmt.Sprint(e.Year))
	}
	if e.Studio != "" {
		extras = append(extras, e.Studio)
	}
	if len(extras) > 0 {
		out += " (" + strings.Join(extras, ", ") + ")"
	}
	return out
}

func byStatus(pool []*entry, status string) []*entry {
	out := []*entry{}
	for _, e := range pool {
		if e.Status == status {
			out = append(out, e)
		}
	}
	return out
}

func sortBucket(in []*entry) {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Rank < in[j].Rank })
}
