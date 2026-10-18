// An in-memory file system for the Go/WASM runtime. Go's syscall package talks to globalThis.fs (the Node.js fs
// callback API); wasm_exec.js only provides a stub that fails every call. This file provides the working subset, so
// the game's own importer can unpack the player's Age of Zombies APK and write the cache it plays from entirely in
// the browser. It also exposes globalThis.aozFS for the page: put the APK in, take the cache out and keep it.
//
// Load it BEFORE wasm_exec.js (which only installs its stub when globalThis.fs is missing).
"use strict";
(() => {
	const O_WRONLY = 1, O_RDWR = 2, O_CREAT = 64, O_EXCL = 128, O_TRUNC = 512, O_APPEND = 1024, O_DIRECTORY = 65536;
	const S_IFDIR = 0o40000, S_IFREG = 0o100000;
	const decoder = new TextDecoder("utf-8");
	let inoCounter = 1;

	const fail = (code, message) => {
		const err = new Error(code + ": " + message);
		err.code = code;
		return err;
	};

	class Node {
		constructor(dir) {
			this.dir = dir;
			this.children = dir ? new Map() : null;
			this.data = dir ? null : new Uint8Array(0);
			this.size = 0;
			this.mode = dir ? 0o755 : 0o644;
			this.mtime = Date.now();
			this.ino = ++inoCounter;
		}
		ensure(capacity) {
			if (this.data.length >= capacity) return;
			const next = new Uint8Array(Math.max(capacity, this.data.length * 2, 4096));
			next.set(this.data.subarray(0, this.size));
			this.data = next;
		}
	}

	const root = new Node(true);
	let cwd = "/";

	function split(path) {
		let full = String(path);
		if (!full.startsWith("/")) full = cwd + "/" + full;
		const parts = [];
		for (const part of full.split("/")) {
			if (part === "" || part === ".") continue;
			if (part === "..") { parts.pop(); continue; }
			parts.push(part);
		}
		return parts;
	}

	function lookup(path) {
		let node = root;
		for (const part of split(path)) {
			if (!node.dir) throw fail("ENOTDIR", path);
			node = node.children.get(part);
			if (!node) throw fail("ENOENT", path);
		}
		return node;
	}

	function parentOf(path) {
		const parts = split(path);
		if (parts.length === 0) throw fail("EEXIST", path);
		const name = parts.pop();
		let node = root;
		for (const part of parts) {
			node = node.children.get(part);
			if (!node) throw fail("ENOENT", path);
			if (!node.dir) throw fail("ENOTDIR", path);
		}
		return { dir: node, name };
	}

	function stat(node) {
		const mode = (node.dir ? S_IFDIR : S_IFREG) | node.mode;
		return {
			dev: 1, ino: node.ino, mode, nlink: 1, uid: 0, gid: 0, rdev: 0, size: node.size, blksize: 4096,
			blocks: Math.ceil(node.size / 512), atimeMs: node.mtime, mtimeMs: node.mtime, ctimeMs: node.mtime,
			isDirectory: () => node.dir, isFile: () => !node.dir, isSymbolicLink: () => false,
		};
	}

	const fds = new Map();
	let nextFD = 3;
	const textBuffers = { 1: "", 2: "" };

	function writeText(fd, bytes) {
		textBuffers[fd] += decoder.decode(bytes);
		const newline = textBuffers[fd].lastIndexOf("\n");
		if (newline !== -1) {
			(fd === 2 ? console.warn : console.log)(textBuffers[fd].substring(0, newline));
			textBuffers[fd] = textBuffers[fd].substring(newline + 1);
		}
	}

	function openFile(path, flags) {
		const parts = split(path);
		let node;
		try {
			node = lookup(path);
			if ((flags & O_CREAT) && (flags & O_EXCL)) throw fail("EEXIST", path);
		} catch (err) {
			if (err.code !== "ENOENT" || !(flags & O_CREAT)) throw err;
			const { dir, name } = parentOf(path);
			node = new Node(false);
			dir.children.set(name, node);
		}
		if ((flags & O_DIRECTORY) && !node.dir) throw fail("ENOTDIR", path);
		const writing = (flags & (O_WRONLY | O_RDWR)) !== 0;
		if (node.dir && writing) throw fail("EISDIR", path);
		if (!node.dir && (flags & O_TRUNC) && writing) { node.size = 0; node.mtime = Date.now(); }
		const fd = nextFD++;
		fds.set(fd, { node, flags, pos: 0, path: parts.join("/") });
		return fd;
	}

	function entry(fd) {
		const file = fds.get(fd);
		if (!file) throw fail("EBADF", String(fd));
		return file;
	}

	function removeNode(path, wantDir) {
		const { dir, name } = parentOf(path);
		const node = dir.children.get(name);
		if (!node) throw fail("ENOENT", path);
		if (wantDir) {
			if (!node.dir) throw fail("ENOTDIR", path);
			if (node.children.size) throw fail("ENOTEMPTY", path);
		} else if (node.dir) {
			throw fail("EISDIR", path);
		}
		dir.children.delete(name);
	}

	// Every call delivers its result through the callback, synchronously (Go waits on a channel the callback fills).
	const call = (callback, work) => {
		let result;
		try {
			result = work();
		} catch (err) {
			callback(err);
			return;
		}
		callback(null, result);
	};

	const fs = {
		constants: { O_RDONLY: 0, O_WRONLY, O_RDWR, O_CREAT, O_EXCL, O_TRUNC, O_APPEND, O_DIRECTORY },
		writeSync(fd, buf) {
			if (fd === 1 || fd === 2) { writeText(fd, buf); return buf.length; }
			throw fail("EBADF", String(fd));
		},
		write(fd, buf, offset, length, position, callback) {
			call(callback, () => {
				const view = buf.subarray(offset, offset + length);
				if (fd === 1 || fd === 2) { writeText(fd, view); return length; }
				const file = entry(fd);
				if (file.node.dir) throw fail("EISDIR", file.path);
				if (!(file.flags & (O_WRONLY | O_RDWR))) throw fail("EBADF", file.path);
				const node = file.node;
				let at = position === null || position === undefined ? file.pos : position;
				if (file.flags & O_APPEND) at = node.size;
				node.ensure(at + length);
				if (at > node.size) node.data.fill(0, node.size, at);
				node.data.set(view, at);
				node.size = Math.max(node.size, at + length);
				node.mtime = Date.now();
				if (position === null || position === undefined) file.pos = at + length;
				return length;
			});
		},
		read(fd, buffer, offset, length, position, callback) {
			call(callback, () => {
				if (fd === 0) return 0;
				const file = entry(fd);
				if (file.node.dir) throw fail("EISDIR", file.path);
				const node = file.node;
				const at = position === null || position === undefined ? file.pos : position;
				const n = Math.max(0, Math.min(length, node.size - at));
				if (n > 0) buffer.set(node.data.subarray(at, at + n), offset);
				if (position === null || position === undefined) file.pos = at + n;
				return n;
			});
		},
		open(path, flags, mode, callback) { call(callback, () => openFile(path, flags)); },
		close(fd, callback) { call(callback, () => { entry(fd); fds.delete(fd); }); },
		fstat(fd, callback) { call(callback, () => stat(entry(fd).node)); },
		stat(path, callback) { call(callback, () => stat(lookup(path))); },
		lstat(path, callback) { call(callback, () => stat(lookup(path))); },
		readdir(path, callback) {
			call(callback, () => {
				const node = lookup(path);
				if (!node.dir) throw fail("ENOTDIR", path);
				return Array.from(node.children.keys());
			});
		},
		mkdir(path, perm, callback) {
			call(callback, () => {
				const { dir, name } = parentOf(path);
				if (dir.children.has(name)) throw fail("EEXIST", path);
				dir.children.set(name, new Node(true));
			});
		},
		unlink(path, callback) { call(callback, () => removeNode(path, false)); },
		rmdir(path, callback) { call(callback, () => removeNode(path, true)); },
		rename(from, to, callback) {
			call(callback, () => {
				const source = parentOf(from);
				const node = source.dir.children.get(source.name);
				if (!node) throw fail("ENOENT", from);
				const target = parentOf(to);
				const existing = target.dir.children.get(target.name);
				if (existing && existing.dir && !node.dir) throw fail("EISDIR", to);
				if (existing && !existing.dir && node.dir) throw fail("ENOTDIR", to);
				if (existing && existing.dir && existing.children.size) throw fail("ENOTEMPTY", to);
				source.dir.children.delete(source.name);
				target.dir.children.set(target.name, node);
			});
		},
		ftruncate(fd, length, callback) {
			call(callback, () => {
				const node = entry(fd).node;
				if (node.dir) throw fail("EISDIR", "");
				node.ensure(length);
				if (length > node.size) node.data.fill(0, node.size, length);
				node.size = length;
			});
		},
		truncate(path, length, callback) {
			call(callback, () => {
				const node = lookup(path);
				if (node.dir) throw fail("EISDIR", path);
				node.ensure(length);
				if (length > node.size) node.data.fill(0, node.size, length);
				node.size = length;
			});
		},
		fsync(fd, callback) { callback(null); },
		chmod(path, mode, callback) { call(callback, () => { lookup(path).mode = mode & 0o7777; }); },
		fchmod(fd, mode, callback) { call(callback, () => { entry(fd).node.mode = mode & 0o7777; }); },
		chown(path, uid, gid, callback) { call(callback, () => { lookup(path); }); },
		fchown(fd, uid, gid, callback) { call(callback, () => { entry(fd); }); },
		lchown(path, uid, gid, callback) { call(callback, () => { lookup(path); }); },
		utimes(path, atime, mtime, callback) { call(callback, () => { lookup(path).mtime = mtime * 1000; }); },
		link(path, link, callback) { callback(fail("ENOSYS", "link")); },
		symlink(path, link, callback) { callback(fail("ENOSYS", "symlink")); },
		readlink(path, callback) { callback(fail("EINVAL", path)); },
	};

	globalThis.fs = fs;
	if (!globalThis.process) {
		globalThis.process = {
			getuid() { return 0; }, getgid() { return 0; }, geteuid() { return 0; }, getegid() { return 0; },
			getgroups() { return []; }, pid: 1, ppid: 0,
			umask() { return 0o22; },
			cwd() { return cwd; },
			chdir(dir) { lookup(dir); cwd = "/" + split(dir).join("/"); },
		};
	}

	// What the page needs: hand files in, take the cache out, drop what is no longer wanted.
	function mkdirs(parts) {
		let node = root;
		for (const part of parts) {
			let next = node.children.get(part);
			if (!next) { next = new Node(true); node.children.set(part, next); }
			node = next;
		}
		return node;
	}

	globalThis.aozFS = {
		writeFile(path, bytes) {
			const parts = split(path);
			const name = parts.pop();
			const dir = mkdirs(parts);
			const node = new Node(false);
			node.data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
			node.size = node.data.length;
			dir.children.set(name, node);
		},
		exists(path) { try { lookup(path); return true; } catch (e) { return false; } },
		readFile(path) { const node = lookup(path); return node.data.subarray(0, node.size); },
		remove(path) {
			try {
				const { dir, name } = parentOf(path);
				dir.children.delete(name);
			} catch (e) { /* already gone */ }
		},
		// Every file below a directory as [path, bytes] (the bytes are not copied).
		list(prefix) {
			const out = [];
			const walk = (node, path) => {
				if (!node.dir) { out.push([path, node.data.subarray(0, node.size)]); return; }
				for (const [name, child] of node.children) walk(child, path + "/" + name);
			};
			try {
				const start = lookup(prefix);
				walk(start, "/" + split(prefix).join("/"));
			} catch (e) { /* nothing there */ }
			return out;
		},
	};
})();
