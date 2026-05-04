package handler

import (
	"future_next_baseball/internal/uow"
)

// CommitOrRollback UnitOfWork를 커밋한다. 실패 시 유저 캐시를
// 무효화하고 (userID가 있는 경우) ActionError를 반환한다.
func CommitOrRollback(u *uow.UnitOfWork) error {
	if err := u.Commit(); err != nil {
		if id := u.UserID(); id != 0 {
			_ = u.Container().UserCache.DeleteAll(id)
		}
		return Errorf(CodeInternalError, "commit failed. err : %v", err.Error())
	}
	return nil
}
