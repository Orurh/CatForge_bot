package app

import (
    "context"
    "errors"
    "crypto/rand"
    "encoding/hex"
    "strings"
    "time"
)

const CatPhotoChangeStars = 10

var (
    ErrCatPhotoInvalid = errors.New("invalid cat photo request")
    ErrCatPhotoPending = errors.New("cat photo request already pending")
    ErrCatPhotoNotFound = errors.New("cat photo request not found")
    ErrCatPhotoConflict = errors.New("cat photo state conflict")
)

type CatPhotoStatus string
const (
    CatPhotoPending CatPhotoStatus = "pending"
    CatPhotoAwaitingPayment CatPhotoStatus = "awaiting_payment"
    CatPhotoApplied CatPhotoStatus = "applied"
    CatPhotoRejected CatPhotoStatus = "rejected"
    CatPhotoSuperseded CatPhotoStatus = "superseded"
    CatPhotoRefunded CatPhotoStatus = "refunded"
)

type CatPhotoRequest struct {
    ID int64
    UserID int64
    CatID int64
    FileID string
    Status CatPhotoStatus
}
type CatPhotoReviewResult struct {
    Request CatPhotoRequest
    NeedsPayment bool
    Applied bool
}
type CatPhotoInvoice struct {
    RequestID int64
    TelegramID int64
    Payload string
    Stars int
    ExpiresAt time.Time
}
type CatPhotoPayment struct {
    TelegramID int64
    Currency string
    Stars int
    Payload string
    ChargeID string
    Refunded bool
}
type CatPhotoRepository interface {
    Submit(ctx context.Context, userID, catID int64, fileID string) (CatPhotoRequest, error)
    Review(ctx context.Context, requestID, reviewerTelegramID int64, approve bool) (CatPhotoReviewResult, error)
    Current(ctx context.Context, userID, catID int64) (string, error)
    OpenForUser(ctx context.Context, userID int64) (*CatPhotoRequest, error)
    TelegramUserID(ctx context.Context, userID int64) (int64, error)
    AwaitingReview(ctx context.Context, limit int) ([]CatPhotoRequest, error)
    CreateInvoice(ctx context.Context, requestID, telegramID int64, payload string, now time.Time) (CatPhotoInvoice, error)
    Checkout(ctx context.Context, queryID string, payment CatPhotoPayment, now time.Time) (bool, error)
    RecordPayment(ctx context.Context, payment CatPhotoPayment, now time.Time) error
}
type CatPhotoService struct {
    repo CatPhotoRepository
    clock Clock
}
func NewCatPhotoService(repo CatPhotoRepository, clock Clock) *CatPhotoService {
    return &CatPhotoService{repo: repo, clock: clock}
}
func ValidCatPhotoFileID(fileID string) bool {
    return len(fileID) > 0 && len(fileID) <= 512 && strings.TrimSpace(fileID) == fileID
}
func (s *CatPhotoService) Submit(ctx context.Context, userID, catID int64, fileID string) (CatPhotoRequest, error) {
    if s == nil || s.repo == nil || userID <= 0 || catID <= 0 || !ValidCatPhotoFileID(fileID) {
        return CatPhotoRequest{}, ErrCatPhotoInvalid
    }
    return s.repo.Submit(ctx, userID, catID, fileID)
}
func (s *CatPhotoService) Review(ctx context.Context, requestID, reviewerID int64, approve bool) (CatPhotoReviewResult, error) {
    if s == nil || s.repo == nil || requestID <= 0 || reviewerID <= 0 { return CatPhotoReviewResult{}, ErrCatPhotoInvalid }
    return s.repo.Review(ctx, requestID, reviewerID, approve)
}
func (s *CatPhotoService) Current(ctx context.Context, userID, catID int64) (string,error) {
    if s == nil || s.repo == nil { return "", nil }
    return s.repo.Current(ctx, userID, catID)
}
func (s *CatPhotoService) AwaitingReview(ctx context.Context) ([]CatPhotoRequest,error) {
    if s == nil || s.repo == nil { return nil,nil }
    return s.repo.AwaitingReview(ctx, 20)
}

func (s *CatPhotoService) OpenForUser(ctx context.Context,userID int64)(*CatPhotoRequest,error){
    if s==nil||s.repo==nil{return nil,nil}
    return s.repo.OpenForUser(ctx,userID)
}
func (s *CatPhotoService) TelegramUserID(ctx context.Context,userID int64)(int64,error){
    if s==nil||s.repo==nil{return 0,ErrCatPhotoInvalid}
    return s.repo.TelegramUserID(ctx,userID)
}
func (s *CatPhotoService) CreateInvoice(ctx context.Context,requestID,telegramID int64)(CatPhotoInvoice,error){
    if s==nil||s.repo==nil||requestID<=0||telegramID<=0{return CatPhotoInvoice{},ErrCatPhotoInvalid}
    var token [24]byte
    if _,err:=rand.Read(token[:]);err!=nil{return CatPhotoInvoice{},err}
    return s.repo.CreateInvoice(ctx,requestID,telegramID,"photo:"+hex.EncodeToString(token[:]),s.clock.Now())
}
func (s *CatPhotoService) Checkout(ctx context.Context,queryID string,p CatPhotoPayment)(bool,error){
    if s==nil||s.repo==nil{return false,nil}
    return s.repo.Checkout(ctx,queryID,p,s.clock.Now())
}
func (s *CatPhotoService) RecordPayment(ctx context.Context,p CatPhotoPayment)error{
    if s==nil||s.repo==nil{return ErrCatPhotoInvalid}
    return s.repo.RecordPayment(ctx,p,s.clock.Now())
}
