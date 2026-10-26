package user

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	Email                 string         `gorm:"uniqueIndex;not null" json:"email"`
	Username              string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash          string         `gorm:"not null" json:"-"`
	LanguagePreference    string         `gorm:"default:en;not null" json:"languagePreference"`
	CardDeckPreference    string         `gorm:"default:french;not null" json:"cardDeckPreference"`
	SoundEnabled          bool           `gorm:"default:true;not null" json:"soundEnabled"`
	MusicEnabled          bool           `gorm:"default:true;not null" json:"musicEnabled"`
	SoundVolume           int            `gorm:"default:70;not null" json:"soundVolume"`
	MusicVolume           int            `gorm:"default:70;not null" json:"musicVolume"`
	AllowNonFriendInvites bool           `gorm:"column:allow_non_friend_invites;not null" json:"allowNonFriendInvites"`
	WalletBalance         int            `gorm:"not null;default:5000" json:"walletBalance"`
	LastLoginAt           *time.Time     `gorm:"column:last_login_at" json:"lastLoginAt,omitempty"`
	LoginStreakDays       int            `gorm:"column:login_streak_days;not null;default:0" json:"loginStreakDays"`
	TotalXP               int            `gorm:"not null;default:0" json:"totalXp"`
	UsernameChangedAt     *time.Time     `gorm:"column:username_changed_at" json:"usernameChangedAt,omitempty"`
	HonorCompletedWeight  float64        `gorm:"column:honor_completed_weight;not null;default:0" json:"-"`
	HonorAbandonedWeight  float64        `gorm:"column:honor_abandoned_weight;not null;default:0" json:"-"`
	HonorDecayedAt        *time.Time     `gorm:"column:honor_decayed_at" json:"-"`
	HonorCompletedTotal   int64          `gorm:"column:honor_completed_total;not null;default:0" json:"-"`
	HonorAbandonedTotal   int64          `gorm:"column:honor_abandoned_total;not null;default:0" json:"-"`
	HonorScoreSnapshot    int            `gorm:"column:honor_score;not null;default:80" json:"-"`
	AvatarKey             *string        `gorm:"column:avatar_key" json:"-"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}
