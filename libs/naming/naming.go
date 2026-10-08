package naming

import "fmt"

// Parser applies a set of naming rules. It is safe for concurrent use.
type Parser struct {
	opts Options

	videoStackRules  []compiledStackRule
	cleanDateTimes   []*regex
	cleanStrings     []*regex
	episodes         []compiledEpisodeExpression
	multipleEpisodes []compiledEpisodeExpression
	audioBookParts   []*regex
	audioBookNames   []*regex
	extraRuleRegexes map[int]*regex
}

type compiledStackRule struct {
	FileStackRule
	re *regex
}

type compiledEpisodeExpression struct {
	EpisodeExpression
	re *regex
}

// New compiles the rules of opts.
func New(opts Options) (*Parser, error) {
	p := &Parser{opts: opts, extraRuleRegexes: map[int]*regex{}}
	var err error
	compileAll := func(patterns []string) []*regex {
		out := make([]*regex, 0, len(patterns))
		for _, pat := range patterns {
			var r *regex
			if r, err = compile(pat, true); err != nil {
				return nil
			}
			out = append(out, r)
		}
		return out
	}
	compileEpisodes := func(exprs []EpisodeExpression) []compiledEpisodeExpression {
		out := make([]compiledEpisodeExpression, 0, len(exprs))
		for _, e := range exprs {
			var r *regex
			if r, err = compile(e.Expression, true); err != nil {
				return nil
			}
			out = append(out, compiledEpisodeExpression{e, r})
		}
		return out
	}
	for _, rule := range opts.VideoStackRules {
		r, cerr := compile(rule.Expression, true)
		if cerr != nil {
			return nil, fmt.Errorf("video stack rule: %w", cerr)
		}
		p.videoStackRules = append(p.videoStackRules, compiledStackRule{rule, r})
	}
	for i, rule := range opts.VideoExtraRules {
		if rule.Type == ExtraRegex {
			r, cerr := compile(rule.Token, true)
			if cerr != nil {
				return nil, fmt.Errorf("extra rule: %w", cerr)
			}
			p.extraRuleRegexes[i] = r
		}
	}
	if p.cleanDateTimes = compileAll(opts.CleanDateTimes); err != nil {
		return nil, fmt.Errorf("clean date time: %w", err)
	}
	if p.cleanStrings = compileAll(opts.CleanStrings); err != nil {
		return nil, fmt.Errorf("clean string: %w", err)
	}
	if p.audioBookParts = compileAll(opts.AudioBookPartsExpressions); err != nil {
		return nil, fmt.Errorf("audiobook parts: %w", err)
	}
	if p.audioBookNames = compileAll(opts.AudioBookNamesExpressions); err != nil {
		return nil, fmt.Errorf("audiobook names: %w", err)
	}
	if p.episodes = compileEpisodes(opts.EpisodeExpressions); err != nil {
		return nil, fmt.Errorf("episode expression: %w", err)
	}
	if p.multipleEpisodes = compileEpisodes(opts.MultipleEpisodeExpressions); err != nil {
		return nil, fmt.Errorf("multiple episode expression: %w", err)
	}
	return p, nil
}

// Default returns a parser with DefaultOptions.
func Default() *Parser {
	p, err := New(DefaultOptions())
	if err != nil {
		panic(err) // the default rules compile
	}
	return p
}

// Options returns the parser's rules.
func (p *Parser) Options() Options { return p.opts }
