package user

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nyelonong/finapimate/oauth"
	"github.com/nyelonong/finapimate/utils"
)

const (
	GENDER_MALE   int = 1
	GENDER_FEMALE int = 2

	RELATION_REQUEST  int = 1
	RELATION_APPROVED int = 2
)

type UserModule struct {
	DBConn *sqlx.DB
}

func NewUserModule(db *sqlx.DB) *UserModule {
	return &UserModule{
		DBConn: db,
	}
}

type User struct {
	ID              int64                  `json:"user_id,omitempty"        db:"user_id"`
	Email           string                 `json:"email"                    db:"email"`
	Name            string                 `json:"name"                     db:"name"`
	Password        string                 `json:"password"                 db:"password"`
	Gender          int                    `json:"gender"                   db:"gender"`
	BirthDate       int64                  `json:"birth_date"`
	BirthDateValid  time.Time              `db:"birth_date"`
	NIK             string                 `json:"nik"                      db:"nik"`
	NIKValid        int                    `json:"nik_valid,omitempty"      db:"nik_valid"`
	MSISDN          string                 `json:"msisdn"                   db:"msisdn"`
	ThresholdAmount float64                `json:"th_amount"                db:"th_amount"`
	CreateTime      time.Time              `json:"create_time,omitempty"    db:"create_time"`
	Photo           string                 `json:"photo,omitempty"          db:"photo"`
	Ewallet         EwalletInquiryResponse `json:"ewallet"`
}

type UserRelation struct {
	FriendID     int64     `json:"friend_id,omitempty"     db:"friend_id"`
	UserIDA      int64     `json:"user_id_a"               db:"user_id_a"`
	UserIDB      int64     `json:"user_id_b"               db:"user_id_b"`
	Status       int       `json:"status,omitempty"        db:"status"`
	CreateTime   time.Time `json:"create_time,omitempty"   db:"create_time"`
	ApprovedTime time.Time `json:"approved_time"           db:"approved_time"`
	UserProfile  User      `json:"user_profile,omitempty"`
}

type EwalletRegister struct {
	CustomerName   string
	DateOfBirth    string
	PrimaryID      string
	MobileNumber   string
	EmailAddress   string
	CompanyCode    string
	CustomerNumber string
}

type EwalletRegisterResponse struct {
	PrimaryID string
	CompanyID string
}

type EwalletInquiry struct {
	CompanyCode string
	PrimaryID   string
}

type EwalletInquiryResponse struct {
	PrimaryID      string
	CustomerNumber string
	CurrencyCode   string
	Balance        string
	CustomerName   string
	DateOfBirth    string
	MobileNumber   string
	EmailAddress   string
}

func (um *UserModule) UserRegister(user User) error {
	// if !user.ValidateNIK() {
	// 	return fmt.Errorf("NIK is not valid.")
	// }

	user.NIKValid = 0
	user.ThresholdAmount = 100000
	user.BirthDateValid = time.Unix(user.BirthDate, 0)

	tx, err := um.DBConn.Beginx()
	if err != nil {
		log.Println(err)
		return err
	}

	if err := user.Insert(tx); err != nil {
		log.Println(err)
		if err := tx.Rollback(); err != nil {
			log.Println(err)
		}
		return err
	}

	register := EwalletRegister{
		CustomerName:   user.Name,
		DateOfBirth:    time.Now().Format("2006-01-02"),
		PrimaryID:      user.Email,
		MobileNumber:   user.MSISDN,
		EmailAddress:   user.Email,
		CompanyCode:    utils.COMPANY_CODE,
		CustomerNumber: fmt.Sprintf("%d", user.ID),
	}

	if _, err := register.Register(); err != nil {
		log.Println(err)
		if err := tx.Rollback(); err != nil {
			log.Println(err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (user *User) Insert(tx *sqlx.Tx) error {
	sqlQuery := `
        INSERT INTO fm_user (
            email,
            name,
            password,
            gender,
            birth_date,
            nik,
            nik_valid,
            msisdn,
            th_amount,
            create_time
        ) VALUES (
            :email,
            :name,
            :password,
            :gender,
            :birth_date,
            :nik,
            :nik_valid,
            :msisdn,
            :th_amount,
            CURRENT_TIMESTAMP
        )
    `
	_, err := tx.NamedExec(sqlQuery, user)
	if err != nil {
		log.Println(err)
		return err
	}

	tx.QueryRow("select CURRVAL('fm_user_user_id_seq')").Scan(&user.ID)

	return nil
}

func (user *User) ValidateNIK() bool {
	nik := user.NIK
	gender := user.Gender
	year, month, day := user.BirthDateValid.Date()

	// Default length of NIK is 16 digits
	if len(nik) != 16 {
		return false
	}

	// Year
	if nik[10:12] != strconv.Itoa(year)[2:4] {
		return false
	}

	// Month
	if nik[8:10] != strconv.Itoa(int(month)) {
		return false
	}

	// Date
	bornDay := nik[6:8]
	if gender == GENDER_MALE {
		if bornDay != strconv.Itoa(day) {
			return false
		}
	} else {
		if bornDay != strconv.Itoa(day-40) {
			return false
		}
	}

	return true
}

func (user *User) UserLogin(um *UserModule) error {
	query := `
        SELECT
			user_id,
			email,
            name,
            gender,
            birth_date,
            nik,
            nik_valid,
            msisdn,
            th_amount,
            create_time
        FROM fm_user
        WHERE email = $1
        AND password = $2
    `
	if err := um.DBConn.Get(user, query, user.Email, user.Password); err != nil {
		log.Println(err)
		return err
	}

	usr := EwalletInquiry{
		CompanyCode: utils.COMPANY_CODE,
		PrimaryID:   fmt.Sprintf("%s", user.Email),
	}

	ewallet, err := usr.Inquiry()
	if err != nil {
		log.Println(err)
		return err
	}

	user.Ewallet = *ewallet

	return nil
}

func (um *UserModule) SearchFriend(user User) ([]User, error) {
	data := make([]User, 0)

	query := `
        SELECT
            email,
            name,
            gender,
            birth_date,
            nik,
            nik_valid,
            msisdn,
            th_amount,
            create_time
        FROM fm_user
        WHERE email = $1
        OR msisdn = $2
    `

	rows, err := um.DBConn.Queryx(query, user.Email, user.MSISDN)
	if err != nil {
		log.Println(err)
		return data, err
	}

	for rows.Next() {
		var usr User
		if err := rows.StructScan(&usr); err != nil {
			log.Println(err)
		} else {
			data = append(data, usr)
		}
	}

	return data, nil
}

func (um *UserModule) AddFriends(ur []UserRelation) error {
	tx, err := um.DBConn.Beginx()
	if err != nil {
		log.Println(err)
		return err
	}

	for _, usr := range ur {
		usr.Status = RELATION_REQUEST

		if err := usr.Insert(tx); err != nil {
			log.Println(err)
			if err := tx.Rollback(); err != nil {
				log.Println(err)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (ur *UserRelation) Insert(tx *sqlx.Tx) error {
	sqlQuery := `
        INSERT INTO fm_user (
            user_id_a,
            user_id_b,
            status,
            create_time
        ) VALUES (
            :user_id_a,
            :user_id_b,
            CURRENT_TIMESTAMP
        )
    `
	_, err := tx.NamedExec(sqlQuery, ur)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (um *UserModule) ApproveFriends(ur []UserRelation) error {
	tx, err := um.DBConn.Beginx()
	if err != nil {
		log.Println(err)
		return err
	}

	for _, usr := range ur {
		usr.Status = RELATION_APPROVED

		if err := usr.Update(tx); err != nil {
			log.Println(err)
			if err := tx.Rollback(); err != nil {
				log.Println(err)
			}
			return err
		}

		if err := usr.InsertApproved(tx); err != nil {
			log.Println(err)
			if err := tx.Rollback(); err != nil {
				log.Println(err)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (ur *UserRelation) Update(tx *sqlx.Tx) error {
	sqlQuery := `
        UPDATE
            fm_friend
        SET
            status          = :status,
            approved_time   = CURRENT_TIMESTAMP
        WHERE friend_id = :friend_id
    `
	_, err := tx.NamedExec(sqlQuery, ur)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (ur *UserRelation) InsertApproved(tx *sqlx.Tx) error {
	sqlQuery := `
        INSERT INTO fm_user (
            user_id_a,
            user_id_b,
            status,
            create_time,
            approved_time
        ) VALUES (
            :user_id_a,
            :user_id_b,
            :status,
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP
        )
    `
	_, err := tx.NamedExec(sqlQuery, ur)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (um *UserModule) FriendRequest(user User) ([]UserRelation, error) {
	data := make([]UserRelation, 0)

	query := `
        SELECT
            friend_id,
            user_id_a,
            user_id_b,
            status,
            create_time
        FROM fm_friend
        WHERE user_id_a = $1
        AND status = $2
    `

	rows, err := um.DBConn.Queryx(query, user.ID, RELATION_REQUEST)
	if err != nil {
		log.Println(err)
		return data, err
	}

	for rows.Next() {
		var usr UserRelation
		var userProfile User
		if err := rows.StructScan(&usr); err != nil {
			log.Println(err)
		} else {
			if err := userProfile.Get(um); err != nil {
				log.Println(err)
			} else {
				usr.UserProfile = userProfile
				data = append(data, usr)
			}
		}
	}

	return data, nil
}

func (u *User) ListFriend(um *UserModule) ([]User, error) {
	// Query for user_id u.ID and only for the approved relation.
	q := `
		SELECT user_id_b
		FROM fm_friend
		WHERE
			user_id_a = $1 AND
			status = $2
	`

	var fids []int64
	err := um.DBConn.Select(&fids, q, u.ID, RELATION_APPROVED)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var flist []User
	for _, fid := range fids {
		f := User{
			ID: fid,
		}

		if err = f.Get(um); err != nil {
			log.Println(err)
			return nil, err
		}

		flist = append(flist, f)
	}

	return flist, nil
}

func (user *User) Get(um *UserModule) error {
	query := `
        SELECT
			email,
            name,
            gender,
            birth_date,
            nik,
            nik_valid,
            msisdn,
            th_amount,
            create_time
        FROM fm_user
        WHERE user_id = $1
    `
	if err := um.DBConn.Get(user, query, user.ID); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (er *EwalletRegister) Register() (*EwalletRegisterResponse, error) {
	encoded, err := json.Marshal(er)
	if err != nil {
		return nil, err
		log.Println(err)
	}

	now := oauth.GetTime()

	method := "POST"
	path := "/ewallet/customers"

	// get access token.
	accessToken, err := oauth.GetAccessToken()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + accessToken
	headers["Origin"] = "tokopedia.com"
	headers["X-BCA-Key"] = utils.API_KEY
	headers["X-BCA-Timestamp"] = now
	headers["X-BCA-Signature"] = utils.GetSignature(method, path, accessToken, string(encoded), now)

	agent := utils.NewHTTPRequest()
	agent.Url = utils.API_URL
	agent.Path = path
	agent.Method = method
	agent.IsJson = true
	agent.Json = er
	agent.Headers = headers

	body, err := agent.DoReq()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var resp EwalletRegisterResponse
	if err := json.Unmarshal(*body, &resp); err != nil {
		log.Println(err)
		var errResp utils.Error
		_ = json.Unmarshal(*body, &errResp)
		log.Println(errResp)
		return nil, err
	}

	return &resp, nil
}

func (ei *EwalletInquiry) Inquiry() (*EwalletInquiryResponse, error) {
	now := oauth.GetTime()
	method := "GET"
	path := "/ewallet/customers/" + ei.CompanyCode + "/" + ei.PrimaryID

	// get access token.
	accessToken, err := oauth.GetAccessToken()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + accessToken
	headers["Origin"] = "tokopedia.com"
	headers["X-BCA-Key"] = utils.API_KEY
	headers["X-BCA-Timestamp"] = now
	headers["X-BCA-Signature"] = utils.GetSignature(method, path, accessToken, "", now)

	agent := utils.NewHTTPRequest()
	agent.Url = utils.API_URL
	agent.Path = path
	agent.Method = method
	agent.IsJson = true
	agent.Json = ""
	agent.Headers = headers

	body, err := agent.DoReq()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var resp EwalletInquiryResponse
	if err := json.Unmarshal(*body, &resp); err != nil {
		log.Println(err)
		var errResp utils.Error
		_ = json.Unmarshal(*body, &errResp)
		log.Println(errResp)
		return nil, err
	}

	return &resp, nil
}
