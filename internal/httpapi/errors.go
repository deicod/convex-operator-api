package httpapi

import (
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writeKubeError(w http.ResponseWriter, err error) {
	if apierrors.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if apierrors.IsAlreadyExists(err) {
		writeError(w, http.StatusConflict, "already exists")
		return
	}
	if apierrors.IsForbidden(err) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if apierrors.IsConflict(err) {
		writeError(w, http.StatusConflict, "conflict")
		return
	}
	if statusErr, ok := err.(*apierrors.StatusError); ok {
		switch statusErr.ErrStatus.Reason {
		case metav1.StatusReasonInvalid, metav1.StatusReasonBadRequest:
			writeError(w, http.StatusBadRequest, statusErr.Error())
			return
		}
	}
	writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", err))
}
