export namespace main {
	
	export class EnhancedQuery {
	    original: string;
	    enhanced: string;
	    search_terms: string[];
	    content_filters: any[];
	    metadata_filters: any[];
	    intent: string;
	    requires_count: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnhancedQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.original = source["original"];
	        this.enhanced = source["enhanced"];
	        this.search_terms = source["search_terms"];
	        this.content_filters = source["content_filters"];
	        this.metadata_filters = source["metadata_filters"];
	        this.intent = source["intent"];
	        this.requires_count = source["requires_count"];
	    }
	}
	export class IndexingStatus {
	    state: string;
	    filesProcessed: number;
	    totalFiles: number;
	    pendingFiles: number;
	    currentFile: string;
	    errors: number;
	    elapsedTime: number;
	
	    static createFrom(source: any = {}) {
	        return new IndexingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.filesProcessed = source["filesProcessed"];
	        this.totalFiles = source["totalFiles"];
	        this.pendingFiles = source["pendingFiles"];
	        this.currentFile = source["currentFile"];
	        this.errors = source["errors"];
	        this.elapsedTime = source["elapsedTime"];
	    }
	}
	export class SearchRequest {
	    query: string;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class SearchResult {
	    id: string;
	    path: string;
	    name: string;
	    type: string;
	    size: number;
	    modifiedAt: string;
	    score: number;
	    highlights: string[];
	    snippet: string;
	    totalResults: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.size = source["size"];
	        this.modifiedAt = source["modifiedAt"];
	        this.score = source["score"];
	        this.highlights = source["highlights"];
	        this.snippet = source["snippet"];
	        this.totalResults = source["totalResults"];
	    }
	}
	export class SearchResponseWithDetails {
	    results: SearchResult[];
	    enhanced_query?: EnhancedQuery;
	    used_llm: boolean;
	    search_time: number;
	    total_count: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResponseWithDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], SearchResult);
	        this.enhanced_query = this.convertValues(source["enhanced_query"], EnhancedQuery);
	        this.used_llm = source["used_llm"];
	        this.search_time = source["search_time"];
	        this.total_count = source["total_count"];
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
	
	export class SystemStatus {
	    status: string;
	    uptime: number;
	    total_files: number;
	    indexed_files: number;
	    pending_files: number;
	    failed_files: number;
	    database: Record<string, any>;
	    embeddings: Record<string, any>;
	    indexing: Record<string, any>;
	    resources: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new SystemStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.uptime = source["uptime"];
	        this.total_files = source["total_files"];
	        this.indexed_files = source["indexed_files"];
	        this.pending_files = source["pending_files"];
	        this.failed_files = source["failed_files"];
	        this.database = source["database"];
	        this.embeddings = source["embeddings"];
	        this.indexing = source["indexing"];
	        this.resources = source["resources"];
	    }
	}

}

