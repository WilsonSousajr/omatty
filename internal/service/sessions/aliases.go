package sessions

import "github.com/WilsonSousajr/omatty/internal/domain/session"

// The record of projects and sessions moved to internal/domain/session (ADR
// 0001, migration step 3.1). These aliases keep every importer compiling
// while the importers move over; migration step 8.1 deletes them. New code
// imports internal/domain/session directly:
//
//	import "github.com/WilsonSousajr/omatty/internal/domain/session"
//	st := session.State{Version: session.Version}

// Version is session.Version.
const Version = session.Version

// Project is session.Project.
type Project = session.Project

// Session is session.Session.
type Session = session.Session

// State is session.State.
type State = session.State

// PlaceholderTitle is session.PlaceholderTitle.
//
//	title := sessions.PlaceholderTitle(id)
func PlaceholderTitle(id string) string { return session.PlaceholderTitle(id) }

// PlaceholderBranch is session.PlaceholderBranch.
//
//	branch := sessions.PlaceholderBranch(id)
func PlaceholderBranch(id string) string { return session.PlaceholderBranch(id) }

// Slug is session.Slug.
//
//	name := sessions.Slug(title)
func Slug(s string) string { return session.Slug(s) }
