package core

import (
	"context"
	"iter"
	"time"
)

// Store gives access to all repositories. Implementations live in libs/store;
// everything else depends only on these interfaces.
//
// Repository methods return errors wrapping ErrNotFound, ErrConflict or
// ErrInvalid where applicable.
type Store interface {
	Libraries() LibraryRepository
	Items() ItemRepository
	MediaSources() MediaSourceRepository
	Images() ImageRepository
	People() PersonRepository
	Users() UserRepository
	UserData() UserDataRepository
	AuthSessions() AuthSessionRepository
	Jobs() JobQueue
	Scans() ScanRepository

	// InTx runs fn in a transaction. The Store passed to fn is bound to the
	// transaction; fn's error rolls it back.
	InTx(ctx context.Context, fn func(tx Store) error) error
}

// LibraryRepository stores libraries.
type LibraryRepository interface {
	Get(ctx context.Context, id ID) (Library, error)
	List(ctx context.Context) ([]Library, error)
	Create(ctx context.Context, lib *Library) error
	Update(ctx context.Context, lib *Library) error
	// Delete removes the library and all its items.
	Delete(ctx context.Context, id ID) error
}

// ItemRepository stores items.
type ItemRepository interface {
	Get(ctx context.Context, id ID) (Item, error)
	GetByPath(ctx context.Context, libraryID ID, path string) (Item, error)
	Query(ctx context.Context, q ItemQuery) (Page[Item], error)
	// Walk streams every item matching q, ignoring Limit and Offset.
	Walk(ctx context.Context, q ItemQuery) iter.Seq2[Item, error]
	// Upsert inserts or replaces items by ID.
	Upsert(ctx context.Context, items ...Item) error
	// Delete removes items together with their descendants, extras, media
	// sources, images, credits and user data.
	Delete(ctx context.Context, ids ...ID) error
	// MarkSeen sets the scan generation of the library's present items
	// whose path is prefix or lies below it; missing items stay missing. A
	// scan calls it for folders it could not read, so their items stay as
	// they were.
	MarkSeen(ctx context.Context, libraryID ID, prefix string, generation int64) error
	// Touch sets the scan generation of the library's items with exactly
	// these paths and clears their missing state, for folders a scan found
	// unchanged.
	Touch(ctx context.Context, libraryID ID, generation int64, paths ...string) error
	// MarkMissing marks the library's items with a path that the scan of
	// the given generation did not see as missing since now, and returns
	// how many it marked.
	MarkMissing(ctx context.Context, libraryID ID, generation int64, now time.Time) (int, error)
	// PurgeMissing deletes the library's items missing since before, with
	// what Delete removes, and returns their IDs.
	PurgeMissing(ctx context.Context, libraryID ID, before time.Time) ([]ID, error)
	// Values lists distinct attribute values (genres, studios, …), ranked by
	// relevance when q.Search is set and by name otherwise.
	Values(ctx context.Context, q ValueQuery) ([]string, error)
}

// MediaSourceRepository stores the media sources of items.
type MediaSourceRepository interface {
	ListForItem(ctx context.Context, itemID ID) ([]MediaSource, error)
	// Replace sets the item's media sources, removing any others.
	Replace(ctx context.Context, itemID ID, sources []MediaSource) error
}

// ImageRepository stores images of items and people.
type ImageRepository interface {
	// Get returns ErrNotFound for unknown images.
	Get(ctx context.Context, id ID) (Image, error)
	ListForOwner(ctx context.Context, ownerID ID) ([]Image, error)
	// ListForOwners lists the images of several owners at once; owners
	// without images are absent.
	ListForOwners(ctx context.Context, ownerIDs []ID) (map[ID][]Image, error)
	// Replace sets the owner's images, removing any others.
	Replace(ctx context.Context, ownerID ID, images []Image) error
}

// PersonRepository stores people and their credits.
type PersonRepository interface {
	Get(ctx context.Context, id ID) (Person, error)
	// FindByName matches the name case-insensitively.
	FindByName(ctx context.Context, name string) (Person, error)
	Upsert(ctx context.Context, people ...Person) error
	// Search lists people ranked by relevance when q.Search is set and by
	// sort name otherwise.
	Search(ctx context.Context, q PersonQuery) ([]Person, error)
	CreditsForItem(ctx context.Context, itemID ID) ([]Credit, error)
	// ReplaceCredits sets the item's credits, removing any others.
	ReplaceCredits(ctx context.Context, itemID ID, credits []Credit) error
}

// UserRepository stores user accounts.
type UserRepository interface {
	Get(ctx context.Context, id ID) (User, error)
	// GetByName matches the name case-insensitively.
	GetByName(ctx context.Context, name string) (User, error)
	List(ctx context.Context) ([]User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id ID) error
}

// AuthSessionRepository stores signed-in clients.
type AuthSessionRepository interface {
	// Create adds a session, replacing the user's session on the same
	// device.
	Create(ctx context.Context, s *AuthSession) error
	// GetByTokenHash returns ErrNotFound for unknown tokens.
	GetByTokenHash(ctx context.Context, hash []byte) (AuthSession, error)
	// ListForUser returns the user's sessions, most recently seen first.
	ListForUser(ctx context.Context, userID ID) ([]AuthSession, error)
	// Touch records activity.
	Touch(ctx context.Context, id ID, at time.Time) error
	Delete(ctx context.Context, id ID) error
}

// UserDataRepository stores per-user item state.
type UserDataRepository interface {
	// Get returns ErrNotFound when the user has no state for the item.
	Get(ctx context.Context, userID, itemID ID) (UserData, error)
	// GetMany returns the state for the given items, omitting items without
	// state.
	GetMany(ctx context.Context, userID ID, itemIDs []ID) (map[ID]UserData, error)
	Put(ctx context.Context, d *UserData) error
}

// ScanRepository stores the state of library scans.
type ScanRepository interface {
	// NextGeneration starts a scan of the library and returns its
	// generation, one more than the last.
	NextGeneration(ctx context.Context, libraryID ID) (int64, error)
	// Folder returns the state a scan recorded for a folder.
	Folder(ctx context.Context, libraryID ID, path string) (FolderState, error)
	// PutFolders records folder states, replacing any for the same paths.
	PutFolders(ctx context.Context, folders ...FolderState) error
	// DeleteFolders removes the library's folder states below generation,
	// left by folders that are gone.
	DeleteFolders(ctx context.Context, libraryID ID, before int64) error
}

// FolderState is what a scan saw of a folder: its identity and
// modification time, which tell the next scan whether to list it again,
// and its entries.
type FolderState struct {
	LibraryID ID
	Path      string
	// ModTime and FileID (inode and device, or the Windows file ID) change
	// when entries are added, removed or renamed.
	ModTime    time.Time
	FileID     string
	Entries    []FolderEntry
	Generation int64
}

// FolderEntry is a file or folder in a folder.
type FolderEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// JobQueue is a durable queue of background jobs.
type JobQueue interface {
	// Enqueue adds a pending job, or does nothing when a pending or running
	// job has the same UniqueKey. It reports whether the job was added.
	Enqueue(ctx context.Context, job *Job) (bool, error)
	// Lease claims the highest-priority due job of one of the kinds for
	// owner until now+ttl. It returns ErrNotFound when no job is due.
	Lease(ctx context.Context, owner string, kinds []string, ttl time.Duration) (Job, error)
	// Extend renews a lease held by owner.
	Extend(ctx context.Context, id ID, owner string, ttl time.Duration) error
	// Complete marks a leased job succeeded.
	Complete(ctx context.Context, id ID, owner string) error
	// Fail records a failed attempt; the job is retried after RetryDelay
	// until MaxAttempts is reached, then marked failed.
	Fail(ctx context.Context, id ID, owner string, cause error) error
}
