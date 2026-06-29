export namespace model {
	
	export class CollectStatus {
	    status: string;
	    message: string;
	    startedAt?: string;
	    finishedAt?: string;
	    exitCode?: number;
	    stdout: string;
	    stderr: string;
	
	    static createFrom(source: any = {}) {
	        return new CollectStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.exitCode = source["exitCode"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	    }
	}
	export class CollectionRun {
	    id: number;
	    device: string;
	    source: string;
	    status: string;
	    message: string;
	    collectedAt: string;
	    command?: string;
	
	    static createFrom(source: any = {}) {
	        return new CollectionRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.device = source["device"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.collectedAt = source["collectedAt"];
	        this.command = source["command"];
	    }
	}
	export class DailyUsage {
	    device: string;
	    source: string;
	    usageDate: string;
	    model: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    reasoningOutputTokens: number;
	    totalTokens: number;
	    costUSD: number;
	    pricingLockedAt?: string;
	    projectPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.source = source["source"];
	        this.usageDate = source["usageDate"];
	        this.model = source["model"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.reasoningOutputTokens = source["reasoningOutputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.costUSD = source["costUSD"];
	        this.pricingLockedAt = source["pricingLockedAt"];
	        this.projectPath = source["projectPath"];
	    }
	}
	export class SessionUsage {
	    device: string;
	    source: string;
	    sessionId: string;
	    lastActivity: string;
	    projectPath: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    reasoningOutputTokens: number;
	    totalTokens: number;
	    costUSD: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.source = source["source"];
	        this.sessionId = source["sessionId"];
	        this.lastActivity = source["lastActivity"];
	        this.projectPath = source["projectPath"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.reasoningOutputTokens = source["reasoningOutputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.costUSD = source["costUSD"];
	    }
	}
	export class DashboardData {
	    daily: DailyUsage[];
	    sessions: SessionUsage[];
	    runs: CollectionRun[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.daily = this.convertValues(source["daily"], DailyUsage);
	        this.sessions = this.convertValues(source["sessions"], SessionUsage);
	        this.runs = this.convertValues(source["runs"], CollectionRun);
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
	
	export class TimeUsage {
	    device: string;
	    source: string;
	    eventTime: string;
	    usageDate: string;
	    model: string;
	    projectPath: string;
	    sessionId: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    reasoningOutputTokens: number;
	    totalTokens: number;
	    costUSD: number;
	
	    static createFrom(source: any = {}) {
	        return new TimeUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.source = source["source"];
	        this.eventTime = source["eventTime"];
	        this.usageDate = source["usageDate"];
	        this.model = source["model"];
	        this.projectPath = source["projectPath"];
	        this.sessionId = source["sessionId"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.reasoningOutputTokens = source["reasoningOutputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.costUSD = source["costUSD"];
	    }
	}
	export class TimeSeriesData {
	    time: TimeUsage[];
	
	    static createFrom(source: any = {}) {
	        return new TimeSeriesData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], TimeUsage);
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

