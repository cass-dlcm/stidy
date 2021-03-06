package db

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/cass-dlcm/pomodoro_tasks/backend/application_errors"
	"github.com/cass-dlcm/pomodoro_tasks/backend/secrets"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/cass-dlcm/pomodoro_tasks/graph/model"
)

var db *sql.DB

func InitDB() error {
	var err error
	user, err := secrets.GetSecret("pomodoro-tasks-db-user")
	if err != nil {
		return err
	}
	password, err := secrets.GetSecret("pomodoro-tasks-db-password")
	if err != nil {
		return err
	}
	host, err := secrets.GetSecret("pomodoro-tasks-db-host")
	if err != nil {
		return err
	}
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@%s/db?parseTime=True", user, password, host))
	if err != nil {
		return err
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	return nil
}

func GetUserUsername(username string) (*model.User, error) {
	user := &model.User{}
	if err := db.QueryRow("select id, username from users where username = ?", username).Scan(&user.ID, &user.Name); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, application_errors.ErrNoUser
	}
	var err error
	user.Lists, err = GetTaskListsUser(user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return user, nil
}

func GetUserAuthUsername(username string) (*model.UserAuth, error) {
	user := &model.UserAuth{}
	if err := db.QueryRow("select username, password from users where username = ?", username).Scan(&user.Name, &user.Password); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, application_errors.ErrNoUser
		}
		return nil, err
	}
	return user, nil
}

func CreateUser(user model.UserAuth) (int64, error) {
	res, err := db.Exec("insert into users (username, password) values (?, ?)", user.Name, user.Password)
	if err != nil {
		return -1, err
	}
	return res.LastInsertId()
}

func GetTaskListsUser(id int64) ([]int64, error) {
	taskLists := []int64{}
	rows, err := db.Query("select todoList from tasklist_user_link where user = ?", id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rowId int64
		if err := rows.Scan(&rowId); err != nil {
			return nil, err
		}
		taskLists = append(taskLists, rowId)
	}
	return taskLists, nil
}

func CreateList(user int64, name string) (*int64, error) {
	res, err := db.Exec("insert into lists (listName) values (?)", name)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	res, err = db.Exec("insert into tasklist_user_link (user, todoList) values (?, ?)", user, id)
	if err != nil {
		return nil, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func GetTodo(id int64) (*model.Todo, error) {
	todo := model.Todo{
		ID:            id,
		DependsOnThis: []*model.TodoStub{},
		ThisDependsOn: []*model.TodoStub{},
	}
	if err := db.QueryRow("select todoName, createdAt, modifiedAt, completedAt, todoList from todos where id = ?", id).Scan(&todo.Name, &todo.CreatedAt, &todo.ModifiedAt, &todo.CompletedAt, &todo.List); err != nil {
		if err == sql.ErrNoRows {
			return nil, application_errors.ErrCannotFetchTodoItem(id, "")
		}
		return nil, err
	}
	rows, err := db.Query("select dependent from dependencies where dependsOn = ?", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	for rows.Next() {
		var todoStubId int64
		if err := rows.Scan(&todoStubId); err != nil {
			return nil, err
		}
		todoStub, err := GetTodoStub(todoStubId)
		if err != nil {
			return nil, err
		}
		todo.DependsOnThis = append(todo.DependsOnThis, todoStub)
	}
	rows, err = db.Query("select dependsOn from dependencies where dependent = ?", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	for rows.Next() {
		var todoStubId int64
		if err := rows.Scan(&todoStubId); err != nil {
			return nil, err
		}
		todoStub, err := GetTodoStub(todoStubId)
		if err != nil {
			return nil, err
		}
		todo.ThisDependsOn = append(todo.ThisDependsOn, todoStub)
	}
	return &todo, nil
}

func GetTodoStub(id int64) (*model.TodoStub, error) {
	todo := model.TodoStub{
		ID: id,
	}
	if err := db.QueryRow("select todoName, completedAt, todoList from todos where id = ?", id).Scan(&todo.Name, &todo.CompletedAt, &todo.List); err != nil {
		return nil, err
	}
	return &todo, nil
}

func GetListOnlyUsers(listId int64) (*model.TaskList, error) {
	taskList := &model.TaskList{
		ID:    listId,
		Users: []int64{},
	}
	if err := db.QueryRow("select listName from lists where id = ?", listId).Scan(&taskList.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, application_errors.ErrCannotFetchTodoList(listId)
		}
		return nil, err
	}
	rows, err := db.Query("select user from tasklist_user_link where todoList = ?", listId)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userid int64
		if err := rows.Scan(&userid); err != nil {
			return nil, err
		}
		taskList.Users = append(taskList.Users, userid)
	}
	return taskList, nil
}

func GetListOnlyTasks(listId int64) (*model.TaskList, error) {
	taskList := &model.TaskList{
		ID:    listId,
		Tasks: []*model.TodoStub{},
	}
	if err := db.QueryRow("select listName from lists where id = ?", listId).Scan(&taskList.Name); err != nil {
		return nil, err
	}
	rows, err := db.Query("select id, todoName, completedAt from todos where todoList = ?", listId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	for rows.Next() {
		todo := &model.TodoStub{
			List: listId,
		}
		if err := rows.Scan(&todo.ID, &todo.Name, &todo.CompletedAt); err != nil {
			return nil, err
		}
		taskList.Tasks = append(taskList.Tasks, todo)
	}
	return taskList, nil
}

func RenameTodo(id int64, name string) (*model.Todo, error) {
	_, err := db.Exec("update todos set todoName = ?, modifiedAt = ? where id = ?", name, time.Now(), id)
	if err != nil {
		return nil, err
	}
	return GetTodo(id)
}

func CreateTodo(todo model.Todo) (*int64, error) {
	res, err := db.Exec("insert into todos (todoName, createdat, modifiedAt, completedAt, todoList) values (?, ?, ?, ?, ?)", todo.Name, todo.CreatedAt, todo.ModifiedAt, todo.CompletedAt, todo.List)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func DeleteTodo(id int64) error {
	_, err := db.Exec("delete from todos where id = ?", id)
	return err
}

func UpdateCompletionTodo(id int64) (*model.Todo, error) {
	_, err := db.Exec("update todos set modifiedAt = ?, completedAt = ? where id = ?", time.Now(), time.Now(), id)
	if err != nil {
		return nil, err
	}
	return GetTodo(id)
}

func CheckDependency(dependent, dependsOn int64) (bool, error) {
	var junk int64
	if err := db.QueryRow("select * from dependencies where dependent = ? and dependsOn = ?", dependent, dependsOn).Scan(&junk, &junk); err != nil {
		return false, err
	}
	return true, nil
}

func AddDependency(dependent, dependsOn int64) ([]*model.Todo, error) {
	if _, err := db.Exec("insert into dependencies (dependent, dependsOn) values (?, ?)", dependent, dependsOn); err != nil {
		return nil, err
	}
	dependentTodo, err := GetTodo(dependent)
	if err != nil {
		return nil, err
	}
	dependsOnTodo, err := GetTodo(dependsOn)
	if err != nil {
		return nil, err
	}
	return []*model.Todo{dependentTodo, dependsOnTodo}, nil
}

func RemoveDependency(dependent, dependsOn int64) (bool, error) {
	if _, err := db.Exec("DELETE FROM dependencies WHERE dependent = ? AND dependsOn = ?", dependent, dependsOn); err != nil {
		return false, err
	}
	return true, nil
}

func CheckSameList(dependent, dependsOn int64) (bool, error) {
	dependentTodo, err := GetTodo(dependent)
	if err != nil {
		return false, err
	}
	dependsOnTodo, err := GetTodo(dependsOn)
	if err != nil {
		return false, err
	}
	return dependentTodo.List == dependsOnTodo.List, nil
}

func GetTimeout(id int64, ipaddr string) (*int, *time.Time, error) {
	var lastFailedLogin time.Time
	var count int
	if err := db.QueryRow("select failed_auth_count, last_failed_auth from user_auth_rate_limits where user_id = ? and ip_addr = ?", id, ipaddr).Scan(&count, &lastFailedLogin); err != nil {
		return nil, nil, err
	}
	return &count, &lastFailedLogin, nil
}

func IncrementTimeout(id int64, ipaddr string, count int) error {
	if _, _, err := GetTimeout(id, ipaddr); errors.Is(err, sql.ErrNoRows) {
		if _, err := db.Exec("insert into user_auth_rate_limits (user_id, ip_addr, failed_auth_count, last_failed_auth) values (?, ?, ?, ?)", id, ipaddr, count, time.Now()); err != nil {
			return err
		}
		return nil
	}
	_, err := db.Exec("update user_auth_rate_limits set failed_auth_count = ?, last_failed_auth = ? where user_id = ? and ip_addr = ?", count, time.Now(), id, ipaddr)
	return err
}

func DeleteTimeout(id int64, ipaddr string) error {
	_, err := db.Exec("delete from user_auth_rate_limits where user_id = ? and ip_addr = ?", id, ipaddr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
