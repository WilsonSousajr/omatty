package forge

import dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"

// The forge vocabulary moved to internal/domain/forge (ADR 0001, migration
// step 3.5). These aliases keep every importer compiling while the importers
// move over; migration step 8.1 deletes them. New code imports
// internal/domain/forge directly:
//
//	import "github.com/WilsonSousajr/omatty/internal/domain/forge"
//	open := pr.State == forge.Open

// PRState is dforge.PRState.
type PRState = dforge.PRState

// CIState is dforge.CIState.
type CIState = dforge.CIState

// PR is dforge.PR.
type PR = dforge.PR

// Review is dforge.Review.
type Review = dforge.Review

// Issue is dforge.Issue.
type Issue = dforge.Issue

// Comment is dforge.Comment.
type Comment = dforge.Comment

// Detail is dforge.Detail.
type Detail = dforge.Detail

// Check is dforge.Check.
type Check = dforge.Check

// Label is dforge.Label.
type Label = dforge.Label

// The states, CI results and review decisions, as dforge declares them.
const (
	Open           = dforge.Open
	Merged         = dforge.Merged
	Closed         = dforge.Closed
	CINone         = dforge.CINone
	CIPassing      = dforge.CIPassing
	CIRunning      = dforge.CIRunning
	CIFailing      = dforge.CIFailing
	ReviewNone     = dforge.ReviewNone
	ReviewRequired = dforge.ReviewRequired
	ReviewApproved = dforge.ReviewApproved
	ReviewChanges  = dforge.ReviewChanges
)

// DetailMax is dforge.DetailMax.
const DetailMax = dforge.DetailMax

// GitHub is dforge.GitHub: how GitHub names its work.
var GitHub = dforge.GitHub

// Neutral is dforge.Neutral: the naming used before a forge is known.
var Neutral = dforge.Neutral
