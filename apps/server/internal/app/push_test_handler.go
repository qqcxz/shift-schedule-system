package app

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// testPushBindingNow sends one synchronous message so the caller immediately sees
// whether the bound Token was accepted. The delivery row is retained even when
// the provider rejects the request, making failures visible in push history.
func (s *Server) testPushBindingNow(w http.ResponseWriter, r *http.Request) error {
	claims := requestClaims(r)
	user, err := findUserByID(r.Context(), s.db, claims.Subject)
	if err != nil {
		return err
	}
	id, err := s.enqueuePush(r.Context(), s.db, claims.StoreID, claims.Subject, pushEventTest, "test:"+uuid.NewString(), nil, "虾推啥推送测试", fmt.Sprintf("%s，您的微信推送 Token 已成功绑定。", user.DisplayName))
	if err != nil {
		return err
	}
	if err := s.processPushDelivery(r.Context(), id); err != nil && !isRecordedPushFailure(err) {
		return err
	}
	delivery, err := s.findPushDelivery(r.Context(), id, claims)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, delivery)
	return nil
}
