package service

import (
	"fmt"

	"lion/pkg/jms-sdk-go/model"
)

func (s *JMService) CreateShareRoom(data model.SharingSessionRequest) (res model.SharingSession, err error) {
	_, err = s.authClient.Post(ShareCreateURL, data, &res)
	return
}

func (s *JMService) JoinShareRoom(data model.SharePostData) (res model.ShareRecord, err error) {
	_, err = s.authClient.Post(ShareSessionJoinURL, data, &res)
	return
}

func (s *JMService) FinishShareRoom(recordId string) (err error) {
	reqUrl := fmt.Sprintf(ShareSessionFinishURL, recordId)
	_, err = s.authClient.Patch(reqUrl, nil, nil)
	return
}
