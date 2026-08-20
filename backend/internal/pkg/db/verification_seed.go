package db

import (
	"time"

	"gorm.io/gorm"

	usermodel "hostsent/backend/internal/modules/user/model"
	verificationmodel "hostsent/backend/internal/modules/verification/model"
)

func seedDemoVerification(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&verificationmodel.VerificationApplication{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	names := []string{"user_east_01", "user_north_01", "user_south_01", "admin"}
	var users []usermodel.User
	if err := tx.Where("username IN ?", names).Find(&users).Error; err != nil {
		return err
	}
	userMap := make(map[string]usermodel.User, len(users))
	for _, user := range users {
		userMap[user.Username] = user
	}
	admin, ok := userMap["admin"]
	if !ok {
		return nil
	}

	now := time.Now()
	reviewedApproved := now.Add(-18 * time.Hour)
	reviewedRejected := now.Add(-6 * time.Hour)
	approvedBy := admin.ID
	rejectedBy := admin.ID
	items := []verificationmodel.VerificationApplication{
		{
			UserID: userMap["user_east_01"].ID, Username: "user_east_01", VerificationType: "personal", Status: "pending",
			RealName: "李东", SubjectName: "李东", IDType: "id_card", IDNumberMasked: "310***********1234", MobileMasked: "139****0001", CountryCode: "CN", RiskFlags: "normal",
			SubmittedAt: now.Add(-2 * time.Hour), Version: 1, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
		},
		{
			UserID: userMap["user_north_01"].ID, Username: "user_north_01", VerificationType: "enterprise", Status: "approved",
			RealName: "王北", SubjectName: "北京北辰科技有限公司", IDType: "business_license", IDNumberMasked: "9111**********88X", MobileMasked: "139****0002", CountryCode: "CN", RiskFlags: "manual_review",
			SubmittedAt: now.Add(-36 * time.Hour), ReviewedAt: &reviewedApproved, ReviewedBy: &approvedBy, ReviewerName: "admin", ReviewNote: "资料齐全，审核通过", Version: 1, CreatedAt: now.Add(-36 * time.Hour), UpdatedAt: reviewedApproved,
		},
		{
			UserID: userMap["user_south_01"].ID, Username: "user_south_01", VerificationType: "personal", Status: "rejected",
			RealName: "陈南", SubjectName: "陈南", IDType: "id_card", IDNumberMasked: "440***********5678", MobileMasked: "139****0003", CountryCode: "CN", RiskFlags: "blurred_document",
			SubmittedAt: now.Add(-12 * time.Hour), ReviewedAt: &reviewedRejected, ReviewedBy: &rejectedBy, ReviewerName: "admin", RejectReasonCode: "document_blur", RejectReason: "证件照片不清晰", ReviewNote: "请重新上传清晰证件照", Version: 1, CreatedAt: now.Add(-12 * time.Hour), UpdatedAt: reviewedRejected,
		},
	}
	if err := tx.Create(&items).Error; err != nil {
		return err
	}
	return nil
}
