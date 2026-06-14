// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storetest provides a shared conformance suite for store.Store
// implementations. Callers wire it with a factory function:
//
//	func TestStore(t *testing.T) {
//	    storetest.Run(t, func(t *testing.T) store.Store { return memory.New() })
//	}
package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quad/quad/internal/store"
	"github.com/quad/quad/pkg/adapter"
)

// Run executes the full behavioral suite against the store returned by open.
// open is called once per sub-test so each sub-test gets a clean store.
func Run(t *testing.T, open func(t *testing.T) store.Store) {
	t.Helper()
	t.Run("Users", func(t *testing.T) { testUsers(t, open(t)) })
	t.Run("Classrooms", func(t *testing.T) { testClassrooms(t, open(t)) })
	t.Run("Assignments", func(t *testing.T) { testAssignments(t, open(t)) })
	t.Run("Roster", func(t *testing.T) { testRoster(t, open(t)) })
	t.Run("Submissions", func(t *testing.T) { testSubmissions(t, open(t)) })
	t.Run("SubmissionErrConflict", func(t *testing.T) { testSubmissionConflict(t, open(t)) })
	t.Run("Grades", func(t *testing.T) { testGrades(t, open(t)) })
	t.Run("GradingRuns", func(t *testing.T) { testGradingRuns(t, open(t)) })
	t.Run("JobIdempotency", func(t *testing.T) { testJobIdempotency(t, open(t)) })
	t.Run("JobClaimOrdering", func(t *testing.T) { testJobClaimOrdering(t, open(t)) })
}

func testUsers(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	u := &store.User{
		ID:           "u1",
		Host:         adapter.HostGitHub,
		HostUserID:   "42",
		HostUsername: "octocat",
		Email:        "octocat@example.com",
	}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := st.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.HostUsername != "octocat" {
		t.Errorf("HostUsername = %q, want octocat", got.HostUsername)
	}

	if _, err := st.GetUser(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetUser missing: got %v, want ErrNotFound", err)
	}

	byUN, err := st.FindUserByHostUsername(ctx, adapter.HostGitHub, "octocat")
	if err != nil {
		t.Fatalf("FindUserByHostUsername: %v", err)
	}
	if byUN.ID != "u1" {
		t.Errorf("FindUserByHostUsername ID = %q, want u1", byUN.ID)
	}

	if _, err := st.FindUserByHostUsername(ctx, adapter.HostGitHub, "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("FindUserByHostUsername missing: got %v, want ErrNotFound", err)
	}

	byID, err := st.FindUserByHostUserID(ctx, adapter.HostGitHub, "42")
	if err != nil {
		t.Fatalf("FindUserByHostUserID: %v", err)
	}
	if byID.ID != "u1" {
		t.Errorf("FindUserByHostUserID ID = %q, want u1", byID.ID)
	}

	if _, err := st.FindUserByHostUserID(ctx, adapter.HostGitHub, "0"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("FindUserByHostUserID missing: got %v, want ErrNotFound", err)
	}
}

func testClassrooms(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	c := &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "cs101-org"}
	if err := st.CreateClassroom(ctx, c); err != nil {
		t.Fatalf("CreateClassroom: %v", err)
	}
	c2 := &store.Classroom{ID: "c2", Name: "CS201", Host: adapter.HostGitHub, HostNamespace: "cs201-org"}
	if err := st.CreateClassroom(ctx, c2); err != nil {
		t.Fatalf("CreateClassroom c2: %v", err)
	}

	got, err := st.GetClassroom(ctx, "c1")
	if err != nil {
		t.Fatalf("GetClassroom: %v", err)
	}
	if got.Name != "CS101" {
		t.Errorf("Name = %q, want CS101", got.Name)
	}

	if _, err := st.GetClassroom(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetClassroom missing: got %v, want ErrNotFound", err)
	}

	list, err := st.ListClassrooms(ctx)
	if err != nil {
		t.Fatalf("ListClassrooms: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListClassrooms count = %d, want 2", len(list))
	}
}

func testAssignments(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})

	deadline := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	a := &store.Assignment{
		ID:          "a1",
		ClassroomID: "c1",
		Title:       "HW1",
		Slug:        "hw-1",
		TemplateRef: adapter.TemplateRef{Host: adapter.HostGitHub, Namespace: "org", Name: "template"},
		Type:        store.AssignmentIndividual,
		Deadline:    &deadline,
		GradingSpec: "grading.json",
	}
	if err := st.CreateAssignment(ctx, a); err != nil {
		t.Fatalf("CreateAssignment: %v", err)
	}

	got, err := st.GetAssignment(ctx, "a1")
	if err != nil {
		t.Fatalf("GetAssignment: %v", err)
	}
	if got.Title != "HW1" {
		t.Errorf("Title = %q, want HW1", got.Title)
	}
	if got.Deadline == nil {
		t.Fatal("Deadline is nil")
	}
	if !got.Deadline.Equal(deadline) {
		t.Errorf("Deadline = %v, want %v", got.Deadline, deadline)
	}

	if _, err := st.GetAssignment(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetAssignment missing: got %v, want ErrNotFound", err)
	}

	got.Title = "HW1-updated"
	got.AccessPolicy = "private"
	if err := st.UpdateAssignment(ctx, got); err != nil {
		t.Fatalf("UpdateAssignment: %v", err)
	}
	re, _ := st.GetAssignment(ctx, "a1")
	if re.Title != "HW1-updated" {
		t.Errorf("after update Title = %q, want HW1-updated", re.Title)
	}

	list, err := st.ListAssignmentsByClassroom(ctx, "c1")
	if err != nil {
		t.Fatalf("ListAssignmentsByClassroom: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("count = %d, want 1", len(list))
	}

	due, err := st.ListAssignmentsDueBy(ctx, deadline.Add(time.Second))
	if err != nil {
		t.Fatalf("ListAssignmentsDueBy: %v", err)
	}
	if len(due) != 1 {
		t.Errorf("ListAssignmentsDueBy count = %d, want 1", len(due))
	}

	notDue, err := st.ListAssignmentsDueBy(ctx, deadline.Add(-time.Second))
	if err != nil {
		t.Fatalf("ListAssignmentsDueBy before deadline: %v", err)
	}
	if len(notDue) != 0 {
		t.Errorf("ListAssignmentsDueBy before deadline count = %d, want 0", len(notDue))
	}
}

func testRoster(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})

	if _, err := st.FindRosterEntryByUsername(ctx, "c1", "octocat"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("FindRosterEntryByUsername empty: got %v, want ErrNotFound", err)
	}

	e := &store.RosterEntry{
		ID:           "r1",
		ClassroomID:  "c1",
		Host:         adapter.HostGitHub,
		HostUsername: "octocat",
		Status:       store.RosterInvited,
	}
	if err := st.CreateRosterEntry(ctx, e); err != nil {
		t.Fatalf("CreateRosterEntry: %v", err)
	}

	got, err := st.FindRosterEntryByUsername(ctx, "c1", "octocat")
	if err != nil {
		t.Fatalf("FindRosterEntryByUsername: %v", err)
	}
	if got.ID != "r1" {
		t.Errorf("ID = %q, want r1", got.ID)
	}

	if _, err := st.GetRosterEntry(ctx, "r1"); err != nil {
		t.Fatalf("GetRosterEntry: %v", err)
	}
	if _, err := st.GetRosterEntry(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetRosterEntry missing: got %v, want ErrNotFound", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	got.Status = store.RosterActive
	got.ClaimedAt = &now
	if err := st.UpdateRosterEntry(ctx, got); err != nil {
		t.Fatalf("UpdateRosterEntry: %v", err)
	}
	re, _ := st.GetRosterEntry(ctx, "r1")
	if re.Status != store.RosterActive {
		t.Errorf("after update Status = %q, want active", re.Status)
	}

	_ = st.CreateRosterEntry(ctx, &store.RosterEntry{ID: "r2", ClassroomID: "c1", Host: adapter.HostGitHub, HostUsername: "monalisa", Status: store.RosterInvited})
	list, err := st.ListRosterEntries(ctx, "c1")
	if err != nil {
		t.Fatalf("ListRosterEntries: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("count = %d, want 2", len(list))
	}
}

func testSubmissions(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	// Seed required foreign-key parents.
	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})
	_ = st.CreateAssignment(ctx, &store.Assignment{ID: "a1", ClassroomID: "c1", Title: "HW1", Slug: "hw-1", TemplateRef: adapter.TemplateRef{Host: adapter.HostGitHub, Namespace: "org", Name: "tpl"}, Type: store.AssignmentIndividual, GradingSpec: "grading.json"})
	_ = st.CreateRosterEntry(ctx, &store.RosterEntry{ID: "r1", ClassroomID: "c1", Host: adapter.HostGitHub, HostUsername: "octocat", Status: store.RosterInvited})

	sub := &store.Submission{
		ID:            "s1",
		AssignmentID:  "a1",
		RosterEntryID: "r1",
		Status:        "provisioning",
	}
	if err := st.CreateSubmission(ctx, sub); err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}

	got, err := st.GetSubmission(ctx, "s1")
	if err != nil {
		t.Fatalf("GetSubmission: %v", err)
	}
	if got.Status != "provisioning" {
		t.Errorf("Status = %q, want provisioning", got.Status)
	}

	if _, err := st.GetSubmission(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetSubmission missing: got %v, want ErrNotFound", err)
	}

	found, err := st.FindSubmission(ctx, "a1", "r1")
	if err != nil {
		t.Fatalf("FindSubmission: %v", err)
	}
	if found.ID != "s1" {
		t.Errorf("FindSubmission ID = %q, want s1", found.ID)
	}

	if _, err := st.FindSubmission(ctx, "a1", "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("FindSubmission missing: got %v, want ErrNotFound", err)
	}

	got.Status = "active"
	got.LastError = "some error"
	got.Repo = adapter.RepoRef{Host: adapter.HostGitHub, Namespace: "org", Name: "hw-1-octocat"}
	if err := st.UpdateSubmission(ctx, got); err != nil {
		t.Fatalf("UpdateSubmission: %v", err)
	}
	re, _ := st.GetSubmission(ctx, "s1")
	if re.Status != "active" {
		t.Errorf("after update Status = %q, want active", re.Status)
	}
	if re.LastError != "some error" {
		t.Errorf("after update LastError = %q, want \"some error\"", re.LastError)
	}

	list, err := st.ListSubmissionsByAssignment(ctx, "a1")
	if err != nil {
		t.Fatalf("ListSubmissionsByAssignment: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("count = %d, want 1", len(list))
	}

	byClass, err := st.ListSubmissionsByClassroom(ctx, "c1")
	if err != nil {
		t.Fatalf("ListSubmissionsByClassroom: %v", err)
	}
	if len(byClass) != 1 {
		t.Errorf("ListSubmissionsByClassroom count = %d, want 1", len(byClass))
	}
}

func testSubmissionConflict(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})
	_ = st.CreateAssignment(ctx, &store.Assignment{ID: "a1", ClassroomID: "c1", Title: "HW1", Slug: "hw-1", TemplateRef: adapter.TemplateRef{Host: adapter.HostGitHub, Namespace: "org", Name: "tpl"}, Type: store.AssignmentIndividual, GradingSpec: "grading.json"})
	_ = st.CreateRosterEntry(ctx, &store.RosterEntry{ID: "r1", ClassroomID: "c1", Host: adapter.HostGitHub, HostUsername: "octocat", Status: store.RosterInvited})

	if err := st.CreateSubmission(ctx, &store.Submission{ID: "s1", AssignmentID: "a1", RosterEntryID: "r1", Status: "provisioning"}); err != nil {
		t.Fatalf("first CreateSubmission: %v", err)
	}
	err := st.CreateSubmission(ctx, &store.Submission{ID: "s2", AssignmentID: "a1", RosterEntryID: "r1", Status: "provisioning"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate CreateSubmission: got %v, want ErrConflict", err)
	}
}

func testGrades(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	// Seed parents.
	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})
	_ = st.CreateAssignment(ctx, &store.Assignment{ID: "a1", ClassroomID: "c1", Title: "HW1", Slug: "hw-1", TemplateRef: adapter.TemplateRef{Host: adapter.HostGitHub, Namespace: "org", Name: "tpl"}, Type: store.AssignmentIndividual, GradingSpec: "grading.json"})
	_ = st.CreateRosterEntry(ctx, &store.RosterEntry{ID: "r1", ClassroomID: "c1", Host: adapter.HostGitHub, HostUsername: "octocat", Status: store.RosterInvited})
	_ = st.CreateSubmission(ctx, &store.Submission{ID: "s1", AssignmentID: "a1", RosterEntryID: "r1", Status: "active"})

	if _, err := st.LatestGradeForSubmission(ctx, "s1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("LatestGrade no grades: got %v, want ErrNotFound", err)
	}

	g1 := &store.Grade{ID: "g1", SubmissionID: "s1", Score: 80, MaxScore: 100, GradedAt: time.Now().UTC().Add(-time.Minute)}
	if err := st.CreateGrade(ctx, g1); err != nil {
		t.Fatalf("CreateGrade g1: %v", err)
	}
	g2 := &store.Grade{ID: "g2", SubmissionID: "s1", Score: 90, MaxScore: 100, GradedAt: time.Now().UTC()}
	if err := st.CreateGrade(ctx, g2); err != nil {
		t.Fatalf("CreateGrade g2: %v", err)
	}

	latest, err := st.LatestGradeForSubmission(ctx, "s1")
	if err != nil {
		t.Fatalf("LatestGrade: %v", err)
	}
	if latest.Score != 90 {
		t.Errorf("Score = %v, want 90", latest.Score)
	}
}

func testGradingRuns(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	// Seed parents.
	_ = st.CreateClassroom(ctx, &store.Classroom{ID: "c1", Name: "CS101", Host: adapter.HostGitHub, HostNamespace: "org"})
	_ = st.CreateAssignment(ctx, &store.Assignment{ID: "a1", ClassroomID: "c1", Title: "HW1", Slug: "hw-1", TemplateRef: adapter.TemplateRef{Host: adapter.HostGitHub, Namespace: "org", Name: "tpl"}, Type: store.AssignmentIndividual, GradingSpec: "grading.json"})
	_ = st.CreateRosterEntry(ctx, &store.RosterEntry{ID: "r1", ClassroomID: "c1", Host: adapter.HostGitHub, HostUsername: "octocat", Status: store.RosterInvited})
	_ = st.CreateSubmission(ctx, &store.Submission{ID: "s1", AssignmentID: "a1", RosterEntryID: "r1", Status: "active"})

	now := time.Now().UTC().Truncate(time.Second)
	run := &store.GradingRun{ID: "gr1", SubmissionID: "s1", Status: "pending", Runner: "container", StartedAt: &now}
	if err := st.CreateGradingRun(ctx, run); err != nil {
		t.Fatalf("CreateGradingRun: %v", err)
	}

	fin := now.Add(10 * time.Second)
	run.Status = "succeeded"
	run.FinishedAt = &fin
	run.Result = []byte(`{"score":90}`)
	if err := st.UpdateGradingRun(ctx, run); err != nil {
		t.Fatalf("UpdateGradingRun: %v", err)
	}

	list, err := st.ListGradingRunsBySubmission(ctx, "s1")
	if err != nil {
		t.Fatalf("ListGradingRunsBySubmission: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("count = %d, want 1", len(list))
	}
	if list[0].Status != "succeeded" {
		t.Errorf("Status = %q, want succeeded", list[0].Status)
	}
}

func testJobIdempotency(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	j := &store.ProvisioningJob{
		ID:             "j1",
		Type:           "create_repo",
		TargetRef:      "s1",
		Status:         store.JobPending,
		IdempotencyKey: "repo:s1",
		ScheduledAt:    time.Now(),
	}
	created, err := st.CreateJob(ctx, j)
	if err != nil || !created {
		t.Fatalf("first CreateJob: created=%v err=%v", created, err)
	}

	// Same idempotency key, different ID → not created, no error.
	dup := &store.ProvisioningJob{
		ID:             "j2",
		Type:           "create_repo",
		TargetRef:      "s1",
		Status:         store.JobPending,
		IdempotencyKey: "repo:s1",
		ScheduledAt:    time.Now(),
	}
	created, err = st.CreateJob(ctx, dup)
	if err != nil || created {
		t.Fatalf("duplicate CreateJob: created=%v err=%v (want created=false, err=nil)", created, err)
	}
}

func testJobClaimOrdering(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()

	t1 := time.Now().Add(-2 * time.Second)
	t2 := time.Now().Add(-1 * time.Second)

	j1 := &store.ProvisioningJob{ID: "j1", Type: "create_repo", TargetRef: "s1", Status: store.JobPending, IdempotencyKey: "k1", ScheduledAt: t1}
	j2 := &store.ProvisioningJob{ID: "j2", Type: "create_repo", TargetRef: "s2", Status: store.JobPending, IdempotencyKey: "k2", ScheduledAt: t2}

	_, _ = st.CreateJob(ctx, j1)
	_, _ = st.CreateJob(ctx, j2)

	claimed, err := st.ClaimNextJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextJob: %v", err)
	}
	if claimed.ID != "j1" {
		t.Errorf("claimed = %s, want j1 (oldest first)", claimed.ID)
	}
	if claimed.Status != store.JobInProgress {
		t.Errorf("Status = %q, want in_progress", claimed.Status)
	}

	// j1 is now in_progress; next claim should return j2.
	claimed2, err := st.ClaimNextJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextJob second: %v", err)
	}
	if claimed2.ID != "j2" {
		t.Errorf("second claimed = %s, want j2", claimed2.ID)
	}

	// Queue exhausted.
	if _, err := st.ClaimNextJob(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ClaimNextJob empty: got %v, want ErrNotFound", err)
	}

	// UpdateJob: mark j1 failed with a retry.
	claimed.Status = store.JobFailed
	claimed.Attempts = 1
	claimed.LastError = "transient error"
	if err := st.UpdateJob(ctx, claimed); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
}
