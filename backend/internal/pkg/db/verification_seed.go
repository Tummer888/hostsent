package db

import (
	"time"

	"gorm.io/gorm"

	usermodel "hostsent/backend/internal/modules/admin/user/account/model"
	verificationmodel "hostsent/backend/internal/modules/admin/user/verification/model"
)

// seedDemoVerification 写入实名认证演示数据（仅空表时）。
//
// 与旧实现的关键差别（doc104 §7）：只对**当前库中真实存在**的用户建单。
// 旧实现无条件按用户名列表建单，用户不存在时 UserID 落 0，制造出
// user_id = 0 的孤儿行（迁移 051 才做了回绑修复）。现在找不到用户就跳过该条，
// 从源头堵住孤儿；若一个都没找到就整体不写，避免又攒一堆孤儿。
func seedDemoVerification(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&verificationmodel.VerificationApplication{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	names := []string{"user_east_01", "user_north_01", "user_south_01"}
	var users []usermodel.User
	if err := tx.Where("username IN ? AND deleted_at IS NULL", names).Find(&users).Error; err != nil {
		return err
	}
	userMap := make(map[string]usermodel.User, len(users))
	for _, user := range users {
		userMap[user.Username] = user
	}
	if len(userMap) == 0 {
		return nil
	}
	admin, err := loadAdminAsUser(tx)
	if err != nil {
		return err
	}

	now := time.Now()
	reviewedApproved := now.Add(-18 * time.Hour)
	reviewedRejected := now.Add(-6 * time.Hour)
	approvedBy := admin.ID
	rejectedBy := admin.ID

	// 逐条按用户是否存在决定是否建单：exists 为 false 的条目整体跳过。
	type demoCase struct {
		username string
		item     verificationmodel.VerificationApplication
	}
	cases := []demoCase{
		{"user_east_01", verificationmodel.VerificationApplication{
			Username: "user_east_01", VerificationType: "personal", Status: "pending",
			RealName: "李东", SubjectName: "李东", IDType: "id_card", IDNumberMasked: "310***********1234", MobileMasked: "139****0001", CountryCode: "CN", RiskFlags: "normal",
			SubmittedAt: now.Add(-2 * time.Hour), Version: 1, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
		}},
		{"user_north_01", verificationmodel.VerificationApplication{
			Username: "user_north_01", VerificationType: "enterprise", Status: "approved",
			RealName: "王北", SubjectName: "北京北辰科技有限公司", IDType: "business_license", IDNumberMasked: "9111**********88X", MobileMasked: "139****0002", CountryCode: "CN", RiskFlags: "manual_review",
			SubmittedAt: now.Add(-36 * time.Hour), ReviewedAt: &reviewedApproved, ReviewedBy: &approvedBy, ReviewerName: "admin", ReviewNote: "资料齐全，审核通过", Provider: "manual", ProviderResult: "passed", Version: 1, CreatedAt: now.Add(-36 * time.Hour), UpdatedAt: reviewedApproved,
		}},
		{"user_south_01", verificationmodel.VerificationApplication{
			Username: "user_south_01", VerificationType: "personal", Status: "rejected",
			RealName: "陈南", SubjectName: "陈南", IDType: "id_card", IDNumberMasked: "440***********5678", MobileMasked: "139****0003", CountryCode: "CN", RiskFlags: "blurred_document",
			SubmittedAt: now.Add(-12 * time.Hour), ReviewedAt: &reviewedRejected, ReviewedBy: &rejectedBy, ReviewerName: "admin", RejectReasonCode: "document_blur", RejectReason: "证件照片不清晰", ReviewNote: "请重新上传清晰证件照", Version: 1, CreatedAt: now.Add(-12 * time.Hour), UpdatedAt: reviewedRejected,
		}},
	}

	var approvedUserID uint64
	for _, c := range cases {
		user, ok := userMap[c.username]
		if !ok {
			continue
		}
		item := c.item
		item.UserID = user.ID
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if item.Status == "approved" {
			approvedUserID = user.ID
		}
	}

	// 已通过的那条同步写实名信任信号：users.real_name_verified_at 是唯一判据
	// （doc104 §5.3），演示数据不写它就会出现「申请单显示已通过、用户却未实名」
	// 的自相矛盾状态，反而误导联调。
	if approvedUserID != 0 {
		if err := tx.Model(&usermodel.User{}).Where("id = ?", approvedUserID).
			Updates(map[string]any{
				"real_name_verified_at":     reviewedApproved,
				"real_name_verified_source": "manual",
			}).Error; err != nil {
			return err
		}
	}
	return nil
}
