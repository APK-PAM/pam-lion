package service

import (
	"lion/pkg/jms-sdk-go/model"
)

func (s *JMService) CheckUserCookie(cookies map[string]string) (user *model.User, err error) {
	client := s.authClient.Clone()
	for k, v := range cookies {
		client.SetCookie(k, v)
	}
	client.SetHeader("X-JMS-LOGIN-TYPE", "T")
	_, err = client.Get(UserProfileURL, &user)
	return
}
