package handler

import (
	"future_cpbl_web_server/internal/errcode"
	"future_cpbl_web_server/internal/uow"
)

// CommitOrRollback UnitOfWork를 커밋한다. 실패 시 유저/클럽 캐시를
// 무효화하고 *errcode.Error를 반환한다.
func CommitOrRollback(u *uow.UnitOfWork) error {
	if err := u.Commit(); err != nil {
		if id := u.UserID(); id != 0 {
			_ = u.Container().UserCache.DeleteAll(id)
		}
		if id := u.ClubID(); id != 0 {
			_ = u.Container().ClubCache.DeleteAll(id)
		}
		return errcode.Newf(errcode.CodeInternalError, "commit failed. err : %v", err.Error())
	}
	return nil
}
