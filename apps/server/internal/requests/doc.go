// Package requests carries out users' requests of movies and seasons of
// series, the problems they report with items, and the discovery of
// titles to request, as Seerr does: a request passes the user's
// permission, rating limit and quota, is approved by an administrator or
// at once, and its approval makes its movie or seasons wanted by the
// acquisition runtime; when they become available, the request completes
// and its user is told. Every step is recorded in the activity log, for
// the notification plugins and the user's own notifications.
package requests
