// SPDX-License-Identifier: GPL-3.0-only
//
// DMZ macOS menu-bar host (prototype, unqualified).
//
// The bundle executable `DMZ` is this host. It owns the Cocoa event loop on the
// main thread, hides from the Dock (LSUIElement + accessory activation policy)
// and supervises the sibling Go core `dmz-core`:
//
//   dmz-core app --no-open --control-file <0700 dir>/control.json
//
// The core writes {origin, token, pid} to the 0600 descriptor. The host keeps
// the bearer token in memory only, never logs it, and talks to loopback only.
// Any command-line arguments are handed to `dmz-core` unchanged (execv), so
// `DMZ version|help|connect|run|...` keeps the current CLI behaviour.
//
// The menu reports what /api/state reports: detected clients and DMZ-launched
// sessions. Detection is NOT protection; the menu never claims masking is
// active for a tool merely because it is installed or detected.

#import <Cocoa/Cocoa.h>
#include <errno.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <signal.h>
#include <spawn.h>
#include <sys/file.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>

extern char **environ;

static NSString *const kCoreName = @"dmz-core";
static const NSTimeInterval kStartTimeout = 15.0;
static const NSTimeInterval kPollInterval = 3.0;

static NSString *DMZCorePath(void) {
  NSString *exe = [[NSBundle mainBundle] executablePath];
  if (exe.length == 0) {
    exe = [[NSProcessInfo processInfo] arguments].firstObject;
  }
  return [[exe stringByDeletingLastPathComponent] stringByAppendingPathComponent:kCoreName];
}

// CLI compatibility: `DMZ <args...>` behaves like the Go binary did.
static void DMZExecCoreIfArguments(int argc, char *argv[]) {
  if (argc < 2) return;
  // LaunchServices may pass -psn_X_Y on older systems; that is not a CLI call.
  if (argc == 2 && strncmp(argv[1], "-psn_", 5) == 0) return;
  NSString *core = DMZCorePath();
  const char *path = core.fileSystemRepresentation;
  argv[0] = (char *)path;
  execv(path, argv);
  fprintf(stderr, "dmz: core executable unavailable\n");
  _exit(127);
}

static BOOL waitExit(pid_t pid, NSTimeInterval seconds) {
  NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:seconds];
  while ([deadline timeIntervalSinceNow] > 0) {
    int status = 0;
    pid_t r = waitpid(pid, &status, WNOHANG);
    if (r == pid || (r < 0 && errno == ECHILD)) return YES;
    usleep(50 * 1000);
  }
  return NO;
}

@interface DMZHost : NSObject <NSApplicationDelegate, NSMenuDelegate>
@end

@implementation DMZHost {
  NSStatusItem *_item;
  NSMenu *_menu;
  NSMenuItem *_statusLine;
  NSMenuItem *_detectedHeader;
  NSMutableArray<NSMenuItem *> *_dynamicItems;
  NSMenuItem *_openItem;
  NSMenuItem *_restartItem;
  NSURLSession *_session;
  NSTimer *_pollTimer;

  NSString *_privateDir;
  NSString *_controlFile;
  NSString *_origin;
  NSString *_token;
  pid_t _corePid;
  dispatch_source_t _exitSource;
  NSMutableArray *_signalSources;
  int _lockFD;
  BOOL _terminating;
}

#pragma mark - Lifecycle

- (void)applicationDidFinishLaunching:(NSNotification *)note {
  [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  _lockFD = -1;
  _dynamicItems = [NSMutableArray array];
  NSURLSessionConfiguration *cfg = [NSURLSessionConfiguration ephemeralSessionConfiguration];
  cfg.connectionProxyDictionary = @{};  // loopback only; never via a proxy
  cfg.timeoutIntervalForRequest = 3;
  cfg.HTTPCookieStorage = nil;
  cfg.URLCache = nil;
  _session = [NSURLSession sessionWithConfiguration:cfg];
  [self installSignalHandlers];
  [self buildMenu];
  if (![self acquireSingleInstanceLock]) {
    // LaunchServices normally routes a second open to this process as a reopen
    // event; this guards direct exec of the bundle binary.
    [self setStatus:@"DMZ is already running"];
    [NSApp terminate:nil];
    return;
  }
  [self startCore];
}

- (BOOL)applicationShouldHandleReopen:(NSApplication *)app hasVisibleWindows:(BOOL)flag {
  // Reopening the app opens settings from the existing core; never a second core.
  [self openSettings:nil];
  return NO;
}

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
  _terminating = YES;
  [self stopCoreGracefully];
  return NSTerminateNow;
}

- (void)installSignalHandlers {
  _signalSources = [NSMutableArray array];
  int sigs[] = {SIGTERM, SIGINT, SIGHUP, SIGQUIT};
  for (size_t i = 0; i < sizeof(sigs) / sizeof(sigs[0]); i++) {
    signal(sigs[i], SIG_IGN);
    dispatch_source_t s = dispatch_source_create(DISPATCH_SOURCE_TYPE_SIGNAL, sigs[i], 0, dispatch_get_main_queue());
    dispatch_source_set_event_handler(s, ^{ [NSApp terminate:nil]; });
    dispatch_resume(s);
    [_signalSources addObject:s];
  }
  signal(SIGPIPE, SIG_IGN);
}

- (BOOL)acquireSingleInstanceLock {
  NSArray *dirs = NSSearchPathForDirectoriesInDomains(NSCachesDirectory, NSUserDomainMask, YES);
  if (dirs.count == 0) return YES;
  NSString *dir = [dirs.firstObject stringByAppendingPathComponent:@"dmz-companion"];
  mkdir(dir.fileSystemRepresentation, 0700);
  chmod(dir.fileSystemRepresentation, 0700);
  NSString *lock = [dir stringByAppendingPathComponent:@"menubar.lock"];
  int fd = open(lock.fileSystemRepresentation, O_RDWR | O_CREAT | O_CLOEXEC | O_NOFOLLOW, 0600);
  if (fd < 0) return YES;  // fail open: warn-and-continue, the core is still private
  if (flock(fd, LOCK_EX | LOCK_NB) != 0) {
    close(fd);
    return NO;
  }
  _lockFD = fd;
  return YES;
}

#pragma mark - Core supervision

- (BOOL)makePrivateDir {
  char tmpl[PATH_MAX];
  NSString *base = NSTemporaryDirectory();  // per-user $TMPDIR on macOS
  snprintf(tmpl, sizeof tmpl, "%s/dmz-host.XXXXXX", base.fileSystemRepresentation);
  if (mkdtemp(tmpl) == NULL) return NO;
  chmod(tmpl, 0700);
  _privateDir = [[NSFileManager defaultManager] stringWithFileSystemRepresentation:tmpl length:strlen(tmpl)];
  _controlFile = [_privateDir stringByAppendingPathComponent:@"control.json"];
  return YES;
}

- (void)startCore {
  [self clearCoreState];
  if (![self makePrivateDir]) {
    [self setStatus:@"DMZ could not create private state"];
    return;
  }
  NSString *core = DMZCorePath();
  if (access(core.fileSystemRepresentation, X_OK) != 0) {
    [self setStatus:@"DMZ core is missing from the app bundle"];
    return;
  }
  NSString *parent = [NSString stringWithFormat:@"%d", getpid()];
  const char *argv[] = {core.fileSystemRepresentation, "app", "--no-open", "--parent-pid", parent.UTF8String, "--control-file",
                        _controlFile.fileSystemRepresentation, NULL};
  posix_spawn_file_actions_t fa;
  posix_spawn_file_actions_init(&fa);
  posix_spawn_file_actions_addopen(&fa, 0, "/dev/null", O_RDONLY, 0);
  posix_spawn_file_actions_addopen(&fa, 1, "/dev/null", O_WRONLY, 0);
  posix_spawn_file_actions_addopen(&fa, 2, "/dev/null", O_WRONLY, 0);
  NSString *home = NSHomeDirectory();
  posix_spawn_file_actions_addchdir_np(&fa, home.fileSystemRepresentation);  // macOS 10.15+
  posix_spawnattr_t attr;
  posix_spawnattr_init(&attr);
  sigset_t def;
  sigemptyset(&def);
  sigaddset(&def, SIGTERM);
  sigaddset(&def, SIGINT);
  sigaddset(&def, SIGHUP);
  sigaddset(&def, SIGQUIT);
  sigaddset(&def, SIGPIPE);
  posix_spawnattr_setsigdefault(&attr, &def);
  sigset_t none;
  sigemptyset(&none);
  posix_spawnattr_setsigmask(&attr, &none);
  // Close every other descriptor (e.g. the instance lock) in the child.
  posix_spawnattr_setflags(&attr, POSIX_SPAWN_SETSIGDEF | POSIX_SPAWN_SETSIGMASK | POSIX_SPAWN_CLOEXEC_DEFAULT);
  pid_t pid = 0;
  int rc = posix_spawn(&pid, argv[0], &fa, &attr, (char *const *)argv, environ);
  posix_spawn_file_actions_destroy(&fa);
  posix_spawnattr_destroy(&attr);
  if (rc != 0) {
    [self setStatus:@"DMZ core could not start"];
    [self removePrivateDir];
    return;
  }
  _corePid = pid;
  [self watchCoreExit:pid];
  [self setStatus:@"DMZ is starting…"];
  [self waitForDescriptorUntil:[NSDate dateWithTimeIntervalSinceNow:kStartTimeout]];
}

- (void)watchCoreExit:(pid_t)pid {
  dispatch_source_t s = dispatch_source_create(DISPATCH_SOURCE_TYPE_PROC, (uintptr_t)pid, DISPATCH_PROC_EXIT, dispatch_get_main_queue());
  __weak DMZHost *weakSelf = self;
  dispatch_source_set_event_handler(s, ^{
    int status = 0;
    waitpid(pid, &status, WNOHANG);
    [weakSelf coreExited:pid];
  });
  dispatch_resume(s);
  _exitSource = s;
  // Covers an exit that happened before the source was armed.
  if (kill(pid, 0) != 0 && errno == ESRCH) {
    int status = 0;
    waitpid(pid, &status, WNOHANG);
    [self coreExited:pid];
  }
}

- (void)coreExited:(pid_t)pid {
  if (pid != _corePid) return;
  if (_exitSource) {
    dispatch_source_cancel(_exitSource);
    _exitSource = nil;
  }
  _corePid = 0;
  [self clearCoreState];
  [self removePrivateDir];
  if (_terminating) return;
  [self setStatus:@"DMZ core stopped"];
  _restartItem.hidden = NO;
  _openItem.enabled = NO;
  [self clearDynamicItems];
}

- (void)waitForDescriptorUntil:(NSDate *)deadline {
  if (_corePid == 0 || _terminating) return;
  if ([self readDescriptor]) {
    [self setStatus:@"DMZ is running"];
    _openItem.enabled = YES;
    _restartItem.hidden = YES;
    [self pollState];
    _pollTimer = [NSTimer scheduledTimerWithTimeInterval:kPollInterval target:self selector:@selector(pollState) userInfo:nil repeats:YES];
    _pollTimer.tolerance = 1.0;
    return;
  }
  if ([deadline timeIntervalSinceNow] <= 0) {
    [self setStatus:@"DMZ core did not become ready"];
    [self terminateCore];
    return;
  }
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.1 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
    [self waitForDescriptorUntil:deadline];
  });
}

- (BOOL)readDescriptor {
  int fd = open(_controlFile.fileSystemRepresentation, O_RDONLY | O_NOFOLLOW | O_CLOEXEC);
  if (fd < 0) return NO;
  struct stat st;
  if (fstat(fd, &st) != 0 || !S_ISREG(st.st_mode) || st.st_uid != getuid() || (st.st_mode & 0077) != 0 || st.st_size > 4096) {
    close(fd);
    return NO;
  }
  char buf[4097];
  ssize_t n = read(fd, buf, 4096);
  close(fd);
  if (n <= 0) return NO;
  NSData *data = [NSData dataWithBytes:buf length:(NSUInteger)n];
  memset(buf, 0, sizeof buf);
  id obj = [NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];
  if (![obj isKindOfClass:[NSDictionary class]]) return NO;  // partially written; retry
  NSString *origin = obj[@"origin"];
  NSString *token = obj[@"token"];
  NSNumber *pid = obj[@"pid"];
  if (![origin isKindOfClass:[NSString class]] || ![token isKindOfClass:[NSString class]] || ![pid isKindOfClass:[NSNumber class]]) return NO;
  if (![origin hasPrefix:@"http://127.0.0.1:"] || token.length != 64 || pid.intValue != _corePid) return NO;
  NSURL *u = [NSURL URLWithString:origin];
  if (u == nil || u.port == nil || u.path.length > 0 || u.query || u.fragment) return NO;
  _origin = origin;
  _token = token;
  return YES;
}

- (void)stopCoreGracefully {
  [_pollTimer invalidate];
  _pollTimer = nil;
  pid_t pid = _corePid;
  if (pid > 0 && _origin && _token) {
    // Ask the core to stop its sessions cleanly; bounded wait, never blocks forever.
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    NSMutableURLRequest *r = [self requestForPath:@"/api/quit" method:@"POST"];
    r.timeoutInterval = 2;
    [[_session dataTaskWithRequest:r completionHandler:^(NSData *d, NSURLResponse *resp, NSError *e) {
      dispatch_semaphore_signal(done);
    }] resume];
    dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
  }
  if (pid > 0 && !waitExit(pid, 3.0)) {
    kill(pid, SIGTERM);
    if (!waitExit(pid, 2.0)) {
      kill(pid, SIGKILL);
      waitExit(pid, 1.0);
    }
  }
  _corePid = 0;
  [self clearCoreState];
  [self removePrivateDir];
  if (_lockFD >= 0) {
    flock(_lockFD, LOCK_UN);
    close(_lockFD);
    _lockFD = -1;
  }
}

- (void)terminateCore {
  pid_t pid = _corePid;
  if (pid > 0) kill(pid, SIGTERM);  // exit source performs cleanup
}

- (void)clearCoreState {
  [_pollTimer invalidate];
  _pollTimer = nil;
  _origin = nil;
  _token = nil;
}

- (void)removePrivateDir {
  if (_privateDir == nil) return;
  // Only our two known names; never a recursive delete of an arbitrary path.
  unlink(_controlFile.fileSystemRepresentation);
  rmdir(_privateDir.fileSystemRepresentation);
  _privateDir = nil;
  _controlFile = nil;
}

#pragma mark - Loopback API

- (NSMutableURLRequest *)requestForPath:(NSString *)path method:(NSString *)method {
  NSURL *url = [NSURL URLWithString:[_origin stringByAppendingString:path]];
  NSMutableURLRequest *r = [NSMutableURLRequest requestWithURL:url];
  r.HTTPMethod = method;
  r.HTTPShouldHandleCookies = NO;
  r.cachePolicy = NSURLRequestReloadIgnoringLocalCacheData;
  [r setValue:[@"Bearer " stringByAppendingString:_token] forHTTPHeaderField:@"Authorization"];
  if ([method isEqualToString:@"POST"]) {
    [r setValue:@"application/json" forHTTPHeaderField:@"Content-Type"];
    r.HTTPBody = [@"{}" dataUsingEncoding:NSUTF8StringEncoding];
  }
  return r;
}

- (void)openSettings:(id)sender {
  if (_origin == nil || _token == nil) {
    if (_corePid == 0 && !_terminating) [self startCore];
    return;
  }
  NSString *origin = _origin;
  [[_session dataTaskWithRequest:[self requestForPath:@"/api/window" method:@"POST"]
               completionHandler:^(NSData *data, NSURLResponse *resp, NSError *err) {
    NSHTTPURLResponse *h = (NSHTTPURLResponse *)resp;
    NSDictionary *obj = (err == nil && h.statusCode == 200 && data.length <= 4096)
        ? [NSJSONSerialization JSONObjectWithData:data options:0 error:NULL] : nil;
    NSString *address = [obj isKindOfClass:[NSDictionary class]] ? obj[@"address"] : nil;
    NSString *prefix = [origin stringByAppendingString:@"/#open="];
    BOOL valid = [address isKindOfClass:[NSString class]] && [address hasPrefix:prefix] && address.length == prefix.length + 64;
    dispatch_async(dispatch_get_main_queue(), ^{
      if (!valid) {
        [self setStatus:@"DMZ settings could not be opened"];
        return;
      }
      [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:address]];
    });
  }] resume];
}

- (void)pollState {
  if (_origin == nil || _token == nil) return;
  [[_session dataTaskWithRequest:[self requestForPath:@"/api/state" method:@"GET"]
               completionHandler:^(NSData *data, NSURLResponse *resp, NSError *err) {
    NSHTTPURLResponse *h = (NSHTTPURLResponse *)resp;
    NSDictionary *obj = (err == nil && h.statusCode == 200 && data.length <= (1u << 20))
        ? [NSJSONSerialization JSONObjectWithData:data options:0 error:NULL] : nil;
    dispatch_async(dispatch_get_main_queue(), ^{
      [self renderState:[obj isKindOfClass:[NSDictionary class]] ? obj : nil];
    });
  }] resume];
}

#pragma mark - Menu

- (void)buildMenu {
  _item = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
  NSImage *icon = nil;
  if (@available(macOS 11.0, *)) {
    icon = [NSImage imageWithSystemSymbolName:@"shield.lefthalf.filled" accessibilityDescription:@"DMZ"];
  }
  if (icon) {
    icon.template = YES;
    _item.button.image = icon;
  } else {
    _item.button.title = @"DMZ";
  }
  _item.button.toolTip = @"DMZ";

  _menu = [[NSMenu alloc] initWithTitle:@"DMZ"];
  _menu.autoenablesItems = NO;
  _menu.delegate = self;
  _statusLine = [[NSMenuItem alloc] initWithTitle:@"DMZ is starting…" action:NULL keyEquivalent:@""];
  _statusLine.enabled = NO;
  [_menu addItem:_statusLine];
  [_menu addItem:[NSMenuItem separatorItem]];
  _detectedHeader = [[NSMenuItem alloc] initWithTitle:@"Tools" action:NULL keyEquivalent:@""];
  _detectedHeader.enabled = NO;
  [_menu addItem:_detectedHeader];
  [_menu addItem:[NSMenuItem separatorItem]];
  _openItem = [[NSMenuItem alloc] initWithTitle:@"Open DMZ Settings…" action:@selector(openSettings:) keyEquivalent:@","];
  _openItem.target = self;
  _openItem.enabled = NO;
  [_menu addItem:_openItem];
  _restartItem = [[NSMenuItem alloc] initWithTitle:@"Restart DMZ Core" action:@selector(restartCore:) keyEquivalent:@""];
  _restartItem.target = self;
  _restartItem.hidden = YES;
  [_menu addItem:_restartItem];
  [_menu addItem:[NSMenuItem separatorItem]];
  NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"Quit DMZ" action:@selector(terminate:) keyEquivalent:@"q"];
  quit.target = NSApp;
  [_menu addItem:quit];
  _item.menu = _menu;
}

- (void)menuWillOpen:(NSMenu *)menu {
  [self pollState];
}

- (void)restartCore:(id)sender {
  if (_corePid != 0) return;
  _restartItem.hidden = YES;
  [self startCore];
}

- (void)setStatus:(NSString *)text {
  _statusLine.title = text;
}

- (void)clearDynamicItems {
  for (NSMenuItem *i in _dynamicItems) [_menu removeItem:i];
  [_dynamicItems removeAllObjects];
  _detectedHeader.title = @"Tools";
}

- (void)addInfo:(NSString *)title {
  NSMenuItem *i = [[NSMenuItem alloc] initWithTitle:title action:NULL keyEquivalent:@""];
  i.enabled = NO;
  i.indentationLevel = 1;
  NSInteger at = [_menu indexOfItem:_detectedHeader] + 1 + (NSInteger)_dynamicItems.count;
  [_menu insertItem:i atIndex:at];
  [_dynamicItems addObject:i];
}

static NSString *DMZSafeLabel(id v, NSUInteger max) {
  if (![v isKindOfClass:[NSString class]]) return @"?";
  NSString *s = [(NSString *)v stringByTrimmingCharactersInSet:[NSCharacterSet controlCharacterSet]];
  return s.length > max ? [s substringToIndex:max] : s;
}

- (void)renderState:(NSDictionary *)state {
  [self clearDynamicItems];
  if (state == nil) {
    [self setStatus:_corePid ? @"DMZ is running (status unavailable)" : @"DMZ core stopped"];
    return;
  }
  [self setStatus:@"DMZ is running"];
  NSArray *clients = [state[@"clients"] isKindOfClass:[NSArray class]] ? state[@"clients"] : @[];
  NSArray *sessions = [state[@"sessions"] isKindOfClass:[NSArray class]] ? state[@"sessions"] : @[];

  // Sessions are the only place a tool is actually routed through DMZ.
  NSMutableDictionary<NSString *, NSMutableArray *> *byClient = [NSMutableDictionary dictionary];
  for (NSDictionary *s in sessions) {
    if (![s isKindOfClass:[NSDictionary class]]) continue;
    NSString *client = DMZSafeLabel(s[@"client"], 32);
    if (!byClient[client]) byClient[client] = [NSMutableArray array];
    [byClient[client] addObject:s];
  }

  NSUInteger detected = 0;
  for (NSDictionary *c in clients) {
    if (![c isKindOfClass:[NSDictionary class]]) continue;
    if (![c[@"available"] isKindOfClass:[NSNumber class]] || ![c[@"available"] boolValue]) continue;
    detected++;
    NSString *cid = DMZSafeLabel(c[@"id"], 32);
    NSString *name = DMZSafeLabel(c[@"name"], 48);
    NSArray *mine = byClient[cid];
    [byClient removeObjectForKey:cid];
    if (mine.count == 0) {
      [self addInfo:[NSString stringWithFormat:@"%@ — detected, not routed through DMZ", name]];
    } else {
      [self addSessionsFor:name sessions:mine];
    }
  }
  for (NSString *cid in byClient) [self addSessionsFor:cid sessions:byClient[cid]];
  _detectedHeader.title = detected ? [NSString stringWithFormat:@"Tools (%lu detected)", (unsigned long)detected] : @"Tools (none detected)";
}

- (void)addSessionsFor:(NSString *)name sessions:(NSArray *)sessions {
  for (NSDictionary *s in sessions) {
    NSString *st = DMZSafeLabel(s[@"state"], 24);
    unsigned long long req = [s[@"requests"] isKindOfClass:[NSNumber class]] ? [s[@"requests"] unsignedLongLongValue] : 0;
    // Reports what the session saw; it is a session count, not a guarantee.
    [self addInfo:[NSString stringWithFormat:@"%@ — DMZ session %@, %llu requests seen", name, st, req]];
  }
}

@end

int main(int argc, char *argv[]) {
  DMZExecCoreIfArguments(argc, argv);
  @autoreleasepool {
    NSApplication *app = [NSApplication sharedApplication];
    DMZHost *host = [DMZHost new];
    app.delegate = host;
    [app run];
  }
  return 0;
}
