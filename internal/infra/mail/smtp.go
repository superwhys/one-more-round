package mail

import (
	"context"
	"fmt"

	"github.com/miebyte/emailsender/api/grpc/emailsendpb"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/grpcutils"
)

type Sender struct {
	emailSender emailsendpb.EmailSenderClient
}

func NewSender() (*Sender, error) {
	emailSenderAddr, err := consulutils.Resolve(context.TODO(), "emailsender")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve emailsender: %w", err)
	}

	conn, err := grpcutils.DialGrpcService(context.TODO(), emailSenderAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial emailsender: %w", err)
	}

	client := emailsendpb.NewEmailSenderClient(conn)
	return &Sender{emailSender: client}, nil
}

func (s *Sender) SendCode(ctx context.Context, to, code string) error {
	body := "又一局登录验证码：" + code + "\r\n10 分钟内有效。若非本人操作，请忽略。\r\n"

	_, err := s.emailSender.Send(ctx, &emailsendpb.SendEmailRequest{
		From:    "系统通知 <omr.superhoven@163.com>",
		To:      to,
		Subject: "One More Round Login Code",
		Body:    body,
	})
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
