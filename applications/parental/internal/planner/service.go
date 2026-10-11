package planner

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	v1 "github.com/liamawhite/parental/gen/parental/v1"
)

// The mutex keeps multi-query snapshots consistent with concurrent mutations.
// SQLite remains the durable source of truth; no plan data is cached here.
type Service struct {
	db *sql.DB
	mu sync.Mutex
}

func New(db *sql.DB) *Service { return &Service{db: db} }
func invalid(message string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(message))
}
func internal(err error) error { return connect.NewError(connect.CodeInternal, err) }
func notFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("item no longer exists; refresh the planner"))
}
func changed(result sql.Result, err error) error {
	if err != nil {
		return internal(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return internal(err)
	}
	if n == 0 {
		return notFound()
	}
	return nil
}

func (s *Service) GetPlan(ctx context.Context, req *connect.Request[v1.GetPlanRequest]) (*connect.Response[v1.GetPlanResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := &v1.GetPlanResponse{}
	p := &v1.Plan{}
	err := s.db.QueryRowContext(ctx, "SELECT birth_date, actual FROM plan WHERE id=1").Scan(&p.BirthDate, &p.Actual)
	if err == nil {
		result.Plan = p
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, internal(err)
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,color,budget_days FROM people ORDER BY name,id")
	if err != nil {
		return nil, internal(err)
	}
	for rows.Next() {
		p := &v1.Person{}
		var budget sql.NullInt32
		if err := rows.Scan(&p.Id, &p.Name, &p.Color, &budget); err != nil {
			rows.Close()
			return nil, internal(err)
		}
		if budget.Valid {
			p.BudgetDays = &budget.Int32
		}
		result.People = append(result.People, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, internal(err)
	}
	rows, err = s.db.QueryContext(ctx, "SELECT id,person_id,start_date,end_date,notes FROM assignments ORDER BY start_date,end_date,id")
	if err != nil {
		return nil, internal(err)
	}
	for rows.Next() {
		a := &v1.Assignment{}
		if err := rows.Scan(&a.Id, &a.PersonId, &a.StartDate, &a.EndDate, &a.Notes); err != nil {
			rows.Close()
			return nil, internal(err)
		}
		result.Assignments = append(result.Assignments, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, internal(err)
	}
	rows, err = s.db.QueryContext(ctx, "SELECT date,name FROM holidays ORDER BY date")
	if err != nil {
		return nil, internal(err)
	}
	for rows.Next() {
		h := &v1.Holiday{}
		if err := rows.Scan(&h.Date, &h.Name); err != nil {
			rows.Close()
			return nil, internal(err)
		}
		result.Holidays = append(result.Holidays, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, internal(err)
	}
	rows, err = s.db.QueryContext(ctx, "SELECT id,name,start_date,end_date FROM vacations ORDER BY start_date,end_date,id")
	if err != nil {
		return nil, internal(err)
	}
	for rows.Next() {
		v := &v1.Vacation{}
		if err := rows.Scan(&v.Id, &v.Name, &v.StartDate, &v.EndDate); err != nil {
			rows.Close()
			return nil, internal(err)
		}
		result.Vacations = append(result.Vacations, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, internal(err)
	}
	Calculate(result)
	return connect.NewResponse(result), nil
}

func (s *Service) UpdatePlan(ctx context.Context, req *connect.Request[v1.UpdatePlanRequest]) (*connect.Response[v1.UpdatePlanResponse], error) {
	p := req.Msg.Plan
	if p == nil {
		return nil, invalid("birth date is required")
	}
	d, err := parseDate(p.BirthDate)
	if err != nil || d.Year() < 1 || d.Year() > 9998 {
		return nil, invalid("enter a valid birth date")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.ExecContext(ctx, `INSERT INTO plan(id,birth_date,actual) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET birth_date=excluded.birth_date,actual=excluded.actual`, p.BirthDate, p.Actual)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.UpdatePlanResponse{}), nil
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Service) SavePerson(ctx context.Context, req *connect.Request[v1.SavePersonRequest]) (*connect.Response[v1.SavePersonResponse], error) {
	p := req.Msg.Person
	if p == nil {
		return nil, invalid("person is required")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 {
		return nil, invalid("name must be between 1 and 100 characters")
	}
	if !colorPattern.MatchString(p.Color) {
		return nil, invalid("choose a valid color")
	}
	if p.BudgetDays != nil && *p.BudgetDays < 0 {
		return nil, invalid("leave budget cannot be negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Id == "" {
		p.Id = uuid.NewString()
		_, err := s.db.ExecContext(ctx, "INSERT INTO people(id,name,color,budget_days) VALUES(?,?,?,?)", p.Id, p.Name, p.Color, p.BudgetDays)
		if err != nil {
			return nil, internal(err)
		}
	} else if err := changed(s.db.ExecContext(ctx, "UPDATE people SET name=?,color=?,budget_days=? WHERE id=?", p.Name, p.Color, p.BudgetDays, p.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SavePersonResponse{Person: p}), nil
}
func (s *Service) DeletePerson(ctx context.Context, req *connect.Request[v1.DeletePersonRequest]) (*connect.Response[v1.DeletePersonResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Msg.Id == "" {
		return nil, invalid("person id is required")
	}
	if err := changed(s.db.ExecContext(ctx, "DELETE FROM people WHERE id=?", req.Msg.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.DeletePersonResponse{}), nil
}
func (s *Service) SaveAssignment(ctx context.Context, req *connect.Request[v1.SaveAssignmentRequest]) (*connect.Response[v1.SaveAssignmentResponse], error) {
	a := req.Msg.Assignment
	if a == nil {
		return nil, invalid("assignment is required")
	}
	start, e1 := parseDate(a.StartDate)
	end, e2 := parseDate(a.EndDate)
	if e1 != nil || e2 != nil || start.Year() < 1 || end.Year() < 1 || end.Before(start) {
		return nil, invalid("enter valid start and end dates, with end on or after start")
	}
	if len(a.Notes) > 10000 {
		return nil, invalid("notes must be at most 10000 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM people WHERE id=?)", a.PersonId).Scan(&exists); err != nil {
		return nil, internal(err)
	}
	if !exists {
		return nil, invalid("choose an existing person")
	}
	if a.Id == "" {
		a.Id = uuid.NewString()
		_, err := s.db.ExecContext(ctx, "INSERT INTO assignments(id,person_id,start_date,end_date,notes) VALUES(?,?,?,?,?)", a.Id, a.PersonId, a.StartDate, a.EndDate, a.Notes)
		if err != nil {
			return nil, internal(err)
		}
	} else if err := changed(s.db.ExecContext(ctx, "UPDATE assignments SET person_id=?,start_date=?,end_date=?,notes=? WHERE id=?", a.PersonId, a.StartDate, a.EndDate, a.Notes, a.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SaveAssignmentResponse{Assignment: a}), nil
}
func (s *Service) DeleteAssignment(ctx context.Context, req *connect.Request[v1.DeleteAssignmentRequest]) (*connect.Response[v1.DeleteAssignmentResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Msg.Id == "" {
		return nil, invalid("assignment id is required")
	}
	if err := changed(s.db.ExecContext(ctx, "DELETE FROM assignments WHERE id=?", req.Msg.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.DeleteAssignmentResponse{}), nil
}

func (s *Service) SaveVacation(ctx context.Context, req *connect.Request[v1.SaveVacationRequest]) (*connect.Response[v1.SaveVacationResponse], error) {
	v := req.Msg.Vacation
	if v == nil {
		return nil, invalid("vacation is required")
	}
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" || len(v.Name) > 200 {
		return nil, invalid("enter a vacation name of at most 200 characters")
	}
	start, e1 := parseDate(v.StartDate)
	end, e2 := parseDate(v.EndDate)
	if e1 != nil || e2 != nil || start.Year() < 1 || end.Year() < 1 || end.Before(start) {
		return nil, invalid("enter valid start and end dates, with end on or after start")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.Id == "" {
		v.Id = uuid.NewString()
		if _, err := s.db.ExecContext(ctx, "INSERT INTO vacations(id,name,start_date,end_date) VALUES(?,?,?,?)", v.Id, v.Name, v.StartDate, v.EndDate); err != nil {
			return nil, internal(err)
		}
	} else if err := changed(s.db.ExecContext(ctx, "UPDATE vacations SET name=?,start_date=?,end_date=? WHERE id=?", v.Name, v.StartDate, v.EndDate, v.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SaveVacationResponse{Vacation: v}), nil
}

func (s *Service) DeleteVacation(ctx context.Context, req *connect.Request[v1.DeleteVacationRequest]) (*connect.Response[v1.DeleteVacationResponse], error) {
	if req.Msg.Id == "" {
		return nil, invalid("vacation id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := changed(s.db.ExecContext(ctx, "DELETE FROM vacations WHERE id=?", req.Msg.Id)); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.DeleteVacationResponse{}), nil
}
