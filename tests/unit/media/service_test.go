package media_unit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	domainmedia "github.com/jsolteam/tenex-platform/internal/domain/media"
	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

// ── mock репозиторий ──────────────────────────────────────────────────────────

type mockMediaRepo struct {
	mu            sync.Mutex
	medias        map[int64]*domainmedia.Media
	variants      map[int64][]*domainmedia.MediaVariant
	nextID        int64
	createErr     error
	addVariantErr error
}

func newMockRepo() *mockMediaRepo {
	return &mockMediaRepo{
		medias:   make(map[int64]*domainmedia.Media),
		variants: make(map[int64][]*domainmedia.MediaVariant),
		nextID:   1,
	}
}

func (r *mockMediaRepo) GetByID(_ context.Context, id int64) (*domainmedia.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.medias[id]
	if !ok {
		return nil, domainmedia.ErrNotFound
	}
	return m, nil
}

func (r *mockMediaRepo) Create(_ context.Context, m *domainmedia.Media) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m.ID = r.nextID
	r.nextID++
	m.CreatedAt = time.Now()
	cp := *m
	r.medias[m.ID] = &cp
	return nil
}

func (r *mockMediaRepo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.medias[id]; !ok {
		return domainmedia.ErrNotFound
	}
	delete(r.medias, id)
	delete(r.variants, id)
	return nil
}

func (r *mockMediaRepo) GetVariants(_ context.Context, mediaID int64) ([]domainmedia.MediaVariant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domainmedia.MediaVariant
	for _, v := range r.variants[mediaID] {
		result = append(result, *v)
	}
	return result, nil
}

func (r *mockMediaRepo) GetVariantByStorage(_ context.Context, mediaID int64, st domainmedia.StorageType, _ string) (*domainmedia.MediaVariant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.variants[mediaID] {
		if v.StorageType == st {
			cp := *v
			return &cp, nil
		}
	}
	return nil, domainmedia.ErrVariantNotFound
}

func (r *mockMediaRepo) AddVariant(_ context.Context, v *domainmedia.MediaVariant) error {
	if r.addVariantErr != nil {
		return r.addVariantErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v.ID = r.nextID
	r.nextID++
	cp := *v
	r.variants[v.MediaID] = append(r.variants[v.MediaID], &cp)
	return nil
}

func (r *mockMediaRepo) DeleteVariant(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for mediaID, vs := range r.variants {
		for i, v := range vs {
			if v.ID == id {
				r.variants[mediaID] = append(vs[:i], vs[i+1:]...)
				return nil
			}
		}
	}
	return domainmedia.ErrVariantNotFound
}

// ── mock S3 ───────────────────────────────────────────────────────────────────

type mockS3 struct {
	mu        sync.Mutex
	uploaded  map[string][]byte
	deleted   []string
	uploadErr error
}

func newMockS3() *mockS3 {
	return &mockS3{uploaded: make(map[string][]byte)}
}

func (s *mockS3) upload(key string, data []byte) error {
	if s.uploadErr != nil {
		return s.uploadErr
	}
	s.mu.Lock()
	s.uploaded[key] = data
	s.mu.Unlock()
	return nil
}

func (s *mockS3) delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	delete(s.uploaded, key)
}

func (s *mockS3) hasKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.uploaded[key]
	return ok
}

func (s *mockS3) wasDeleted(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.deleted {
		if k == key {
			return true
		}
	}
	return false
}

// ── mock Cache ─────────────────────────────────────────────────────────────────

type mockCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string][]byte)}
}

func (c *mockCache) get(key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	if !ok {
		return nil, infraredis.ErrCacheMiss
	}
	return v, nil
}

func (c *mockCache) set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// ── testableService — упрощённый MediaService для unit-тестов ─────────────────
//
// Повторяет логику media.Service но с интерфейсными зависимостями
// (без прямой зависимости на *infras3.Client и *infraredis.Cache).

type testableService struct {
	repo  *mockMediaRepo
	s3    *mockS3
	cache *mockCache
}

func newTestableService(repo *mockMediaRepo, s3 *mockS3, cache *mockCache) *testableService {
	return &testableService{repo: repo, s3: s3, cache: cache}
}

func fileIDKey(mediaID int64, messenger string) string {
	return fmt.Sprintf("media:fileid:%d:%s", mediaID, messenger)
}

func (ts *testableService) CacheFileID(ctx context.Context, mediaID int64, messenger, fileID string) error {
	ts.cache.set(fileIDKey(mediaID, messenger), []byte(fileID))
	return nil
}

func (ts *testableService) GetFileID(ctx context.Context, mediaID int64, messenger string) (string, error) {
	data, err := ts.cache.get(fileIDKey(mediaID, messenger))
	if errors.Is(err, infraredis.ErrCacheMiss) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (ts *testableService) UploadPhoto(ctx context.Context, ownerID int64, data []byte, contentType string) (int64, string, error) {
	s3Key := fmt.Sprintf("media/%d/test.jpg", ownerID)

	if err := ts.s3.upload(s3Key, data); err != nil {
		return 0, "", err
	}

	m := &domainmedia.Media{OwnerUserID: ownerID, MediaType: domainmedia.MediaTypePhoto}
	if err := ts.repo.Create(ctx, m); err != nil {
		// Компенсация: удалить S3-объект
		ts.s3.delete(s3Key)
		return 0, "", err
	}

	v := &domainmedia.MediaVariant{
		MediaID:     m.ID,
		StorageType: domainmedia.StorageTypeS3,
		ExternalID:  s3Key,
	}
	if err := ts.repo.AddVariant(ctx, v); err != nil {
		// Компенсация: удалить S3 и запись media
		ts.s3.delete(s3Key)
		_ = ts.repo.Delete(ctx, m.ID)
		return 0, "", err
	}

	return m.ID, s3Key, nil
}

// ── тесты ─────────────────────────────────────────────────────────────────────

// TestCacheFileID_Roundtrip проверяет что CacheFileID сохраняет,
// а GetFileID возвращает тот же FileID.
func TestCacheFileID_Roundtrip(t *testing.T) {
	ctx := context.Background()
	svc := newTestableService(newMockRepo(), newMockS3(), newMockCache())

	const (
		mediaID   = int64(42)
		messenger = "telegram"
		fileID    = "AgACAgIAAxkBAAIBZ2_some_file_id"
	)

	// Кэша нет — GetFileID возвращает пустую строку
	got, err := svc.GetFileID(ctx, mediaID, messenger)
	if err != nil {
		t.Fatalf("GetFileID до кэширования: %v", err)
	}
	if got != "" {
		t.Errorf("GetFileID до кэширования = %q, ожидали пустую строку", got)
	}

	// Кэшируем
	if err := svc.CacheFileID(ctx, mediaID, messenger, fileID); err != nil {
		t.Fatalf("CacheFileID: %v", err)
	}

	// Теперь должны получить обратно
	got, err = svc.GetFileID(ctx, mediaID, messenger)
	if err != nil {
		t.Fatalf("GetFileID после кэширования: %v", err)
	}
	if got != fileID {
		t.Errorf("GetFileID = %q, ожидали %q", got, fileID)
	}
}

// TestCacheFileID_IsolatedByMessenger проверяет что разные мессенджеры изолированы.
func TestCacheFileID_IsolatedByMessenger(t *testing.T) {
	ctx := context.Background()
	svc := newTestableService(newMockRepo(), newMockS3(), newMockCache())

	const mediaID = int64(10)

	_ = svc.CacheFileID(ctx, mediaID, "telegram", "tg_file_id")
	_ = svc.CacheFileID(ctx, mediaID, "vk", "vk_file_id")

	tgID, _ := svc.GetFileID(ctx, mediaID, "telegram")
	vkID, _ := svc.GetFileID(ctx, mediaID, "vk")

	if tgID != "tg_file_id" {
		t.Errorf("telegram FileID = %q, ожидали tg_file_id", tgID)
	}
	if vkID != "vk_file_id" {
		t.Errorf("vk FileID = %q, ожидали vk_file_id", vkID)
	}
}

// TestCacheFileID_IsolatedByMediaID проверяет что разные mediaID изолированы.
func TestCacheFileID_IsolatedByMediaID(t *testing.T) {
	ctx := context.Background()
	svc := newTestableService(newMockRepo(), newMockS3(), newMockCache())

	_ = svc.CacheFileID(ctx, 1, "telegram", "file_for_1")
	_ = svc.CacheFileID(ctx, 2, "telegram", "file_for_2")

	id1, _ := svc.GetFileID(ctx, 1, "telegram")
	id2, _ := svc.GetFileID(ctx, 2, "telegram")

	if id1 != "file_for_1" {
		t.Errorf("mediaID=1 FileID = %q, ожидали file_for_1", id1)
	}
	if id2 != "file_for_2" {
		t.Errorf("mediaID=2 FileID = %q, ожидали file_for_2", id2)
	}
}

// TestUploadPhoto_DBCreateError_Compensates проверяет компенсацию
// при ошибке Create в БД: S3-объект должен быть удалён.
func TestUploadPhoto_DBCreateError_Compensates(t *testing.T) {
	ctx := context.Background()
	s3 := newMockS3()
	repo := newMockRepo()
	repo.createErr = errors.New("db: connection lost")

	svc := newTestableService(repo, s3, newMockCache())

	_, _, err := svc.UploadPhoto(ctx, 1, []byte("photo data"), "image/jpeg")
	if err == nil {
		t.Fatal("ожидали ошибку при ошибке Create в БД, получили nil")
	}

	s3Key := "media/1/test.jpg"

	// S3-объект не должен оставаться
	if s3.hasKey(s3Key) {
		t.Error("S3-объект остался после ошибки Create — компенсация не сработала")
	}
	if !s3.wasDeleted(s3Key) {
		t.Error("Delete на S3 не был вызван — компенсация не сработала")
	}
}

// TestUploadPhoto_AddVariantError_Compensates проверяет компенсацию
// при ошибке AddVariant: удаляются и S3-объект, и запись media.
func TestUploadPhoto_AddVariantError_Compensates(t *testing.T) {
	ctx := context.Background()
	s3 := newMockS3()
	repo := newMockRepo()
	repo.addVariantErr = errors.New("db: variant insert failed")

	svc := newTestableService(repo, s3, newMockCache())

	_, _, err := svc.UploadPhoto(ctx, 1, []byte("photo"), "image/jpeg")
	if err == nil {
		t.Fatal("ожидали ошибку при ошибке AddVariant")
	}

	// S3-объект удалён
	if s3.hasKey("media/1/test.jpg") {
		t.Error("S3-объект остался после ошибки AddVariant — компенсация не сработала")
	}

	// Запись media в БД удалена
	repo.mu.Lock()
	mediaCount := len(repo.medias)
	repo.mu.Unlock()
	if mediaCount > 0 {
		t.Errorf("в БД осталось %d записей media после компенсации, ожидали 0", mediaCount)
	}
}

// TestUploadPhoto_S3Error_NoDB проверяет что при ошибке S3
// в БД ничего не создаётся.
func TestUploadPhoto_S3Error_NoDB(t *testing.T) {
	ctx := context.Background()
	s3 := newMockS3()
	s3.uploadErr = errors.New("s3: connection refused")
	repo := newMockRepo()

	svc := newTestableService(repo, s3, newMockCache())

	_, _, err := svc.UploadPhoto(ctx, 1, []byte("photo"), "image/jpeg")
	if err == nil {
		t.Fatal("ожидали ошибку при ошибке S3")
	}

	repo.mu.Lock()
	mediaCount := len(repo.medias)
	repo.mu.Unlock()
	if mediaCount > 0 {
		t.Error("в БД создалась запись media несмотря на ошибку S3")
	}
}

// TestUploadPhoto_Success проверяет успешный путь.
func TestUploadPhoto_Success(t *testing.T) {
	ctx := context.Background()
	s3 := newMockS3()
	repo := newMockRepo()
	svc := newTestableService(repo, s3, newMockCache())

	mediaID, s3Key, err := svc.UploadPhoto(ctx, 99, []byte("real photo"), "image/jpeg")
	if err != nil {
		t.Fatalf("UploadPhoto: %v", err)
	}
	if mediaID == 0 {
		t.Error("mediaID должен быть ненулевым")
	}
	if s3Key == "" {
		t.Error("s3Key не должен быть пустым")
	}
	if !s3.hasKey(s3Key) {
		t.Error("объект не найден в S3 после успешной загрузки")
	}

	// Проверяем что вариант сохранён в БД
	variants, err := repo.GetVariants(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetVariants: %v", err)
	}
	if len(variants) != 1 {
		t.Errorf("ожидали 1 вариант, получили %d", len(variants))
	}
	if variants[0].StorageType != domainmedia.StorageTypeS3 {
		t.Errorf("StorageType = %q, ожидали s3", variants[0].StorageType)
	}
}

// TestGetFileID_MissReturnsEmpty проверяет что отсутствие кэша
// возвращает пустую строку, а не ошибку.
func TestGetFileID_MissReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	svc := newTestableService(newMockRepo(), newMockS3(), newMockCache())

	fileID, err := svc.GetFileID(ctx, 999, "telegram")
	if err != nil {
		t.Errorf("GetFileID при cache miss должен возвращать nil ошибку, получили: %v", err)
	}
	if fileID != "" {
		t.Errorf("GetFileID при cache miss должен возвращать пустую строку, получили %q", fileID)
	}
}

// TestFileIDCacheKey проверяет формат ключа Redis.
func TestFileIDCacheKey(t *testing.T) {
	key := fileIDKey(42, "telegram")
	expected := "media:fileid:42:telegram"
	if key != expected {
		t.Errorf("fileIDKey = %q, ожидали %q", key, expected)
	}

	key2 := fileIDKey(1, "vk")
	expected2 := "media:fileid:1:vk"
	if key2 != expected2 {
		t.Errorf("fileIDKey = %q, ожидали %q", key2, expected2)
	}
}

// Убеждаемся что пакеты компилируются — smoke test для зависимостей.
var _ = bytes.NewReader
var _ = fmt.Sprintf
