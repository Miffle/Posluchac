export namespace audio {
	
	export class TrackInfo {
	    infoHash: string;
	    filePath: string;
	    fileName: string;
	    coverPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new TrackInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.infoHash = source["infoHash"];
	        this.filePath = source["filePath"];
	        this.fileName = source["fileName"];
	        this.coverPath = source["coverPath"];
	    }
	}
	export class Status {
	    state: string;
	    track?: TrackInfo;
	    positionSec: number;
	    durationSec: number;
	    volume: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.track = this.convertValues(source["track"], TrackInfo);
	        this.positionSec = source["positionSec"];
	        this.durationSec = source["durationSec"];
	        this.volume = source["volume"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace library {
	
	export class Track {
	    id: number;
	    infoHash: string;
	    filePath: string;
	    fileName: string;
	    title: string;
	    artist: string;
	    album: string;
	    year: number;
	    genre: string;
	    durationSec: number;
	    sizeBytes: number;
	    coverPath?: string;
	    favourite: boolean;
	    // Go type: time
	    addedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.infoHash = source["infoHash"];
	        this.filePath = source["filePath"];
	        this.fileName = source["fileName"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.year = source["year"];
	        this.genre = source["genre"];
	        this.durationSec = source["durationSec"];
	        this.sizeBytes = source["sizeBytes"];
	        this.coverPath = source["coverPath"];
	        this.favourite = source["favourite"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Playlist {
	    id: number;
	    name: string;
	    // Go type: time
	    createdAt: any;
	    tracks?: Track[];
	
	    static createFrom(source: any = {}) {
	        return new Playlist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.tracks = this.convertValues(source["tracks"], Track);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class PendingTorrent {
	    infoHash: string;
	    name: string;
	    files: torrent.FileInfo[];
	
	    static createFrom(source: any = {}) {
	        return new PendingTorrent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.infoHash = source["infoHash"];
	        this.name = source["name"];
	        this.files = this.convertValues(source["files"], torrent.FileInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace search {
	
	export class Result {
	    title: string;
	    magnetUri: string;
	    torrentUrl: string;
	    seeders: number;
	    leechers: number;
	    size: string;
	    source: string;
	    category: string;
	    infoHash?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.magnetUri = source["magnetUri"];
	        this.torrentUrl = source["torrentUrl"];
	        this.seeders = source["seeders"];
	        this.leechers = source["leechers"];
	        this.size = source["size"];
	        this.source = source["source"];
	        this.category = source["category"];
	        this.infoHash = source["infoHash"];
	    }
	}

}

export namespace sources {
	
	export class Source {
	    id: number;
	    name: string;
	    url: string;
	    itemCount: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.itemCount = source["itemCount"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace torrent {
	
	export class DownloadProgress {
	    infoHash: string;
	    name: string;
	    downloaded: number;
	    total: number;
	    percentDone: number;
	    downloadSpeed: number;
	    uploadSpeed: number;
	    seeders: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.infoHash = source["infoHash"];
	        this.name = source["name"];
	        this.downloaded = source["downloaded"];
	        this.total = source["total"];
	        this.percentDone = source["percentDone"];
	        this.downloadSpeed = source["downloadSpeed"];
	        this.uploadSpeed = source["uploadSpeed"];
	        this.seeders = source["seeders"];
	        this.state = source["state"];
	    }
	}
	export class FileInfo {
	    path: string;
	    name: string;
	    size: number;
	    infoHash: string;
	    bytesCompleted: number;
	    percentDone: number;
	    isSelected: boolean;
	    isComplete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.infoHash = source["infoHash"];
	        this.bytesCompleted = source["bytesCompleted"];
	        this.percentDone = source["percentDone"];
	        this.isSelected = source["isSelected"];
	        this.isComplete = source["isComplete"];
	    }
	}

}

export namespace updater {
	
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	    releaseName: string;
	    releaseNotes: string;
	    releaseUrl: string;
	    downloadUrl: string;
	    assetSize: number;
	    publishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	        this.releaseName = source["releaseName"];
	        this.releaseNotes = source["releaseNotes"];
	        this.releaseUrl = source["releaseUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.assetSize = source["assetSize"];
	        this.publishedAt = source["publishedAt"];
	    }
	}

}

