export namespace downloader {
	
	export class DownloadRequest {
	    url: string;
	    type: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.type = source["type"];
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

