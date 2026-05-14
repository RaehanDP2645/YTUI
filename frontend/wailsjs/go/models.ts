export namespace downloader {
	
	export class BatchDownloadRequest {
	    filePath: string;
	    type: string;
	    quality: string;
	    outputDir: string;
	    parallel: number;
	    skipErrors: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchDownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.type = source["type"];
	        this.quality = source["quality"];
	        this.outputDir = source["outputDir"];
	        this.parallel = source["parallel"];
	        this.skipErrors = source["skipErrors"];
	    }
	}
	export class BatchDownloadResult {
	    message: string;
	    total: number;
	    completed: number;
	    failed: number;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchDownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class DownloadRequest {
	    url: string;
	    type: string;
	    quality: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.type = source["type"];
	        this.quality = source["quality"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class DownloadResult {
	    message: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.outputDir = source["outputDir"];
	    }
	}

}

