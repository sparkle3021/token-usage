export namespace model {
	
	export class AppConfig {
	    autoSyncSeconds: number;
	    ccSwitchDBPath: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoSyncSeconds = source["autoSyncSeconds"];
	        this.ccSwitchDBPath = source["ccSwitchDBPath"];
	    }
	}
	export class BalanceDetail {
	    currency: string;
	    total: number;
	    granted?: number;
	    toppedUp?: number;
	
	    static createFrom(source: any = {}) {
	        return new BalanceDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currency = source["currency"];
	        this.total = source["total"];
	        this.granted = source["granted"];
	        this.toppedUp = source["toppedUp"];
	    }
	}
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
	export class ConfigField {
	    key: string;
	    label: string;
	    type: string;
	    placeholder: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.placeholder = source["placeholder"];
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
	    requestCount: number;
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
	        this.requestCount = source["requestCount"];
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
	    model: string;
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
	        this.model = source["model"];
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
	    deviceNames: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new DashboardData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.daily = this.convertValues(source["daily"], DailyUsage);
	        this.sessions = this.convertValues(source["sessions"], SessionUsage);
	        this.deviceNames = source["deviceNames"];
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
	export class DeviceInfo {
	    deviceId: string;
	    hostname: string;
	    displayName: string;
	    isLocal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.hostname = source["hostname"];
	        this.displayName = source["displayName"];
	        this.isLocal = source["isLocal"];
	    }
	}
	export class HourUsage {
	    device: string;
	    source: string;
	    usageDate: string;
	    hour: number;
	    model: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    reasoningOutputTokens: number;
	    totalTokens: number;
	    costUSD: number;
	    requestCount: number;
	
	    static createFrom(source: any = {}) {
	        return new HourUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.source = source["source"];
	        this.usageDate = source["usageDate"];
	        this.hour = source["hour"];
	        this.model = source["model"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.reasoningOutputTokens = source["reasoningOutputTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.costUSD = source["costUSD"];
	        this.requestCount = source["requestCount"];
	    }
	}
	export class ImportResult {
	    hours: number;
	    daily: number;
	    sessions: number;
	    devices: number;
	    importedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hours = source["hours"];
	        this.daily = source["daily"];
	        this.sessions = source["sessions"];
	        this.devices = source["devices"];
	        this.importedAt = source["importedAt"];
	    }
	}
	export class ModelRanking {
	    model: string;
	    totalTokens: number;
	    costUSD: number;
	    requestCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelRanking(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.totalTokens = source["totalTokens"];
	        this.costUSD = source["costUSD"];
	        this.requestCount = source["requestCount"];
	    }
	}
	export class PricingUpdateResult {
	    litellm: number;
	    message: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PricingUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.litellm = source["litellm"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class ProviderSchema {
	    id: string;
	    planName: string;
	    displayType: string;
	    slotsLabels?: string[];
	    balanceLabel?: string;
	    fields: ConfigField[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderSchema(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.planName = source["planName"];
	        this.displayType = source["displayType"];
	        this.slotsLabels = source["slotsLabels"];
	        this.balanceLabel = source["balanceLabel"];
	        this.fields = this.convertValues(source["fields"], ConfigField);
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
	export class QuotaConfig {
	    id: number;
	    provider: string;
	    plan: string;
	    displayName: string;
	    seq: number;
	    configJson?: string;
	    isValid: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new QuotaConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.plan = source["plan"];
	        this.displayName = source["displayName"];
	        this.seq = source["seq"];
	        this.configJson = source["configJson"];
	        this.isValid = source["isValid"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class QuotaSlot {
	    label: string;
	    usagePercent: number;
	    resetInSec: number;
	
	    static createFrom(source: any = {}) {
	        return new QuotaSlot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.usagePercent = source["usagePercent"];
	        this.resetInSec = source["resetInSec"];
	    }
	}
	export class QuotaData {
	    configId: number;
	    provider: string;
	    plan: string;
	    name: string;
	    slots?: QuotaSlot[];
	    balance?: number;
	    balanceDetails?: BalanceDetail[];
	    error?: string;
	    fetchedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new QuotaData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configId = source["configId"];
	        this.provider = source["provider"];
	        this.plan = source["plan"];
	        this.name = source["name"];
	        this.slots = this.convertValues(source["slots"], QuotaSlot);
	        this.balance = source["balance"];
	        this.balanceDetails = this.convertValues(source["balanceDetails"], BalanceDetail);
	        this.error = source["error"];
	        this.fetchedAt = source["fetchedAt"];
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
	    hour: HourUsage[];
	    deviceNames: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new TimeSeriesData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], TimeUsage);
	        this.hour = this.convertValues(source["hour"], HourUsage);
	        this.deviceNames = source["deviceNames"];
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

