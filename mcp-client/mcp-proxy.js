#!/usr/bin/env node

/**
 * MCP-over-HTTP Proxy with Environment Variable Support
 * Converts HTTP requests to MCP protocol for remote MCP servers
 * Passes configuration via environment variables from Claude Desktop
 * 
 * Usage:
 * 1. Run your swagger-docs-mcp in SSE mode on remote server (without hardcoded config)
 * 2. Run this proxy locally: node mcp-proxy.js http://remote-server:8080
 * 3. Configure Claude Desktop with environment variables
 * 
 * Environment Variables:
 * - WX_MCP_API_KEY: Weather API key
 * - WX_MCP_PACKAGE_ID: Package IDs filter
 * - WX_MCP_TWC_DOMAIN: TWC domain filter
 * - WX_MCP_TWC_PORTFOLIO: TWC portfolio filter  
 * - WX_MCP_TWC_GEOGRAPHY: TWC geography filter
 * - WX_MCP_FILTER_CUSTOM_FIELD: Custom field filter
 */

const http = require('http');
const https = require('https');
const { URL } = require('url');

const REMOTE_SSE_URL = process.argv[2] || 'http://localhost:8080';

if (!process.argv[2]) {
  console.error('Usage: node mcp-proxy.js <remote-sse-url>');
  console.error('Example: node mcp-proxy.js http://your-server:8080');
  console.error('');
  console.error('Environment Variables:');
  console.error('  WX_MCP_API_KEY            - Weather API key');
  console.error('  WX_MCP_PACKAGE_ID         - Package IDs filter');
  console.error('  WX_MCP_TWC_DOMAIN         - TWC domain filter');
  console.error('  WX_MCP_TWC_PORTFOLIO      - TWC portfolio filter');
  console.error('  WX_MCP_TWC_GEOGRAPHY      - TWC geography filter');
  console.error('  WX_MCP_FILTER_CUSTOM_FIELD - Custom field filter');
  console.error('  WX_MCP_DEBUG              - Enable debug logging (true/false)');
  console.error('  WX_MCP_VERBOSE            - Enable verbose logging (true/false)');
  process.exit(1);
}

// Extract configuration from environment variables
const proxyConfig = {
  apiKey: process.env.WX_MCP_API_KEY,
  packageId: process.env.WX_MCP_PACKAGE_ID,
  twcDomain: process.env.WX_MCP_TWC_DOMAIN,
  twcPortfolio: process.env.WX_MCP_TWC_PORTFOLIO,
  twcGeography: process.env.WX_MCP_TWC_GEOGRAPHY,
  filterCustomField: process.env.WX_MCP_FILTER_CUSTOM_FIELD,
  debug: process.env.WX_MCP_DEBUG === 'true',
  verbose: process.env.WX_MCP_VERBOSE === 'true' || process.env.WX_MCP_DEBUG === 'true'
};

// Logging utilities
const log = {
  timestamp: () => new Date().toISOString(),
  info: (msg, ...args) => {
    console.error(`[${log.timestamp()}] [INFO] ${msg}`, ...args);
  },
  debug: (msg, ...args) => {
    if (proxyConfig.debug) {
      console.error(`[${log.timestamp()}] [DEBUG] ${msg}`, ...args);
    }
  },
  verbose: (msg, ...args) => {
    if (proxyConfig.verbose) {
      console.error(`[${log.timestamp()}] [VERBOSE] ${msg}`, ...args);
    }
  },
  error: (msg, ...args) => {
    console.error(`[${log.timestamp()}] [ERROR] ${msg}`, ...args);
  },
  warn: (msg, ...args) => {
    console.error(`[${log.timestamp()}] [WARN] ${msg}`, ...args);
  }
};

log.info('MCP Proxy starting with configuration:');
log.info(`  Remote URL: ${REMOTE_SSE_URL}`);
log.debug(`  API Key: ${proxyConfig.apiKey ? '[REDACTED]' : 'Not set'}`);
log.debug(`  Package ID: ${proxyConfig.packageId || 'Not set'}`);
log.debug(`  TWC Domain: ${proxyConfig.twcDomain || 'Not set'}`);
log.debug(`  TWC Portfolio: ${proxyConfig.twcPortfolio || 'Not set'}`);
log.debug(`  TWC Geography: ${proxyConfig.twcGeography || 'Not set'}`);
log.debug(`  Custom Field: ${proxyConfig.filterCustomField || 'Not set'}`);
log.debug(`  Debug: ${proxyConfig.debug}`);
log.debug(`  Verbose: ${proxyConfig.verbose}`);

class MCPProxy {
  constructor(remoteUrl) {
    this.remoteUrl = new URL(remoteUrl);
    this.tools = [];
    this.initialized = false;
    this.requestCount = 0;
    this.toolCallCount = 0;
    
    log.verbose(`MCPProxy initialized with remote URL: ${remoteUrl}`);
  }

  async fetchTools() {
    const startTime = Date.now();
    log.verbose('Starting tool fetch from remote server');
    
    try {
      // Build query parameters for filtering
      const params = new URLSearchParams();
      
      if (proxyConfig.packageId) {
        params.append('package-ids', proxyConfig.packageId);
        log.verbose(`Added package-ids filter: ${proxyConfig.packageId}`);
      }
      if (proxyConfig.twcDomain) {
        params.append('twc-domains', proxyConfig.twcDomain);
        log.verbose(`Added twc-domains filter: ${proxyConfig.twcDomain}`);
      }
      if (proxyConfig.twcPortfolio) {
        params.append('twc-portfolios', proxyConfig.twcPortfolio);
        log.verbose(`Added twc-portfolios filter: ${proxyConfig.twcPortfolio}`);
      }
      if (proxyConfig.twcGeography) {
        params.append('twc-geographies', proxyConfig.twcGeography);
        log.verbose(`Added twc-geographies filter: ${proxyConfig.twcGeography}`);
      }
      if (proxyConfig.filterCustomField) {
        params.append('filter-custom', proxyConfig.filterCustomField);
        log.verbose(`Added custom filter: ${proxyConfig.filterCustomField}`);
      }
      
      const queryString = params.toString();
      const url = queryString ? `${this.remoteUrl.origin}/tools?${queryString}` : `${this.remoteUrl.origin}/tools`;
      
      log.debug(`Fetching tools from: ${url}`);
      log.verbose(`Query parameters: ${queryString || 'none'}`);
      
      const response = await this.httpRequest(url);
      log.verbose(`Received response from tools endpoint (${response.length} bytes)`);
      
      const data = JSON.parse(response);
      this.tools = data.tools || [];
      
      const duration = Date.now() - startTime;
      log.info(`Successfully fetched ${this.tools.length} tools in ${duration}ms`);
      
      if (proxyConfig.verbose && this.tools.length > 0) {
        log.verbose('Sample tools:');
        this.tools.slice(0, 3).forEach((tool, i) => {
          log.verbose(`  ${i + 1}. ${tool.name} - ${tool.description.substring(0, 50)}...`);
        });
      }
      
    } catch (error) {
      const duration = Date.now() - startTime;
      log.error(`Failed to fetch tools after ${duration}ms: ${error.message}`);
      log.verbose(`Error stack: ${error.stack}`);
    }
  }

  httpRequest(url, options = {}) {
    const requestId = `req-${++this.requestCount}`;
    const startTime = Date.now();
    
    log.verbose(`[${requestId}] Starting HTTP request: ${options.method || 'GET'} ${url}`);
    if (options.headers) {
      log.verbose(`[${requestId}] Headers:`, JSON.stringify(options.headers, null, 2));
    }
    if (options.body) {
      log.verbose(`[${requestId}] Body length: ${options.body.length} bytes`);
      if (proxyConfig.verbose) {
        try {
          const bodyObj = JSON.parse(options.body);
          log.verbose(`[${requestId}] Body content:`, JSON.stringify(bodyObj, null, 2));
        } catch (e) {
          log.verbose(`[${requestId}] Body content (non-JSON): ${options.body.substring(0, 200)}...`);
        }
      }
    }
    
    return new Promise((resolve, reject) => {
      const urlObj = new URL(url);
      const lib = urlObj.protocol === 'https:' ? https : http;
      
      const req = lib.request(urlObj, options, (res) => {
        log.verbose(`[${requestId}] Response received: ${res.statusCode} ${res.statusMessage}`);
        log.verbose(`[${requestId}] Response headers:`, JSON.stringify(res.headers, null, 2));
        
        let data = '';
        let bytesReceived = 0;
        
        res.on('data', chunk => {
          data += chunk;
          bytesReceived += chunk.length;
          log.verbose(`[${requestId}] Received ${chunk.length} bytes (total: ${bytesReceived})`);
        });
        
        res.on('end', () => {
          const duration = Date.now() - startTime;
          log.verbose(`[${requestId}] Request completed in ${duration}ms`);
          
          if (res.statusCode >= 200 && res.statusCode < 300) {
            log.debug(`[${requestId}] HTTP ${res.statusCode} success (${bytesReceived} bytes)`);
            resolve(data);
          } else {
            log.error(`[${requestId}] HTTP ${res.statusCode} error: ${data}`);
            reject(new Error(`HTTP ${res.statusCode}: ${data}`));
          }
        });
      });
      
      req.on('error', (error) => {
        const duration = Date.now() - startTime;
        log.error(`[${requestId}] Request failed after ${duration}ms: ${error.message}`);
        reject(error);
      });
      
      if (options.body) {
        req.write(options.body);
      }
      req.end();
    });
  }

  async handleMCPRequest(request) {
    const requestId = `mcp-${++this.requestCount}`;
    const startTime = Date.now();
    const { method, params, id } = request;

    log.verbose(`[${requestId}] Handling MCP request: ${method} (id: ${id})`);
    if (proxyConfig.verbose && params) {
      log.verbose(`[${requestId}] Request params:`, JSON.stringify(params, null, 2));
    }

    // Handle notifications (no response needed)
    if (method && method.startsWith('notifications/')) {
      log.debug(`[${requestId}] Received notification: ${method}`);
      log.verbose(`[${requestId}] Notifications require no response`);
      return null; // No response for notifications
    }

    switch (method) {
      case 'initialize':
        log.info(`[${requestId}] Initializing MCP proxy`);
        log.verbose(`[${requestId}] Client info:`, params?.clientInfo);
        log.verbose(`[${requestId}] Client capabilities:`, params?.capabilities);
        
        await this.fetchTools();
        this.initialized = true;
        
        const initResponse = {
          jsonrpc: '2.0',
          id,
          result: {
            protocolVersion: '2024-11-05',
            capabilities: {
              tools: { listChanged: true }
            },
            serverInfo: {
              name: 'swagger-docs-mcp-proxy',
              version: '1.0.0'
            }
          }
        };
        
        const duration = Date.now() - startTime;
        log.info(`[${requestId}] Initialization completed in ${duration}ms`);
        log.verbose(`[${requestId}] Init response:`, JSON.stringify(initResponse, null, 2));
        
        return initResponse;

      case 'tools/list':
        log.info(`[${requestId}] Listing available tools`);
        
        if (!this.initialized) {
          log.verbose(`[${requestId}] Proxy not initialized, fetching tools first`);
          await this.fetchTools();
        }
        
        const toolsResponse = {
          jsonrpc: '2.0',
          id,
          result: {
            tools: this.tools.map(tool => ({
              name: tool.name,
              description: tool.description,
              inputSchema: tool.inputSchema
            }))
          }
        };
        
        const listDuration = Date.now() - startTime;
        log.info(`[${requestId}] Returned ${this.tools.length} tools in ${listDuration}ms`);
        
        if (proxyConfig.verbose) {
          log.verbose(`[${requestId}] Tool names:`, this.tools.map(t => t.name).slice(0, 10));
          if (this.tools.length > 10) {
            log.verbose(`[${requestId}] ... and ${this.tools.length - 10} more tools`);
          }
        }
        
        return toolsResponse;

      case 'tools/call':
        const toolCallId = `call-${++this.toolCallCount}`;
        try {
          const { name, arguments: args } = params;
          
          log.info(`[${requestId}] [${toolCallId}] Executing tool: ${name}`);
          log.verbose(`[${requestId}] [${toolCallId}] Original arguments:`, JSON.stringify(args, null, 2));
          
          // Inject API key and configuration into tool arguments
          const enhancedArgs = { ...args };
          let injectionsCount = 0;
          
          // Add API key if available and not already present
          if (proxyConfig.apiKey && !enhancedArgs.apiKey) {
            enhancedArgs.apiKey = proxyConfig.apiKey;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected API key`);
          }
          
          // Add filtering parameters that might be needed for tool execution
          if (proxyConfig.packageId && !enhancedArgs.packageId) {
            enhancedArgs.packageId = proxyConfig.packageId;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected packageId: ${proxyConfig.packageId}`);
          }
          
          if (proxyConfig.twcDomain && !enhancedArgs.twcDomain) {
            enhancedArgs.twcDomain = proxyConfig.twcDomain;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected twcDomain: ${proxyConfig.twcDomain}`);
          }
          
          if (proxyConfig.twcPortfolio && !enhancedArgs.twcPortfolio) {
            enhancedArgs.twcPortfolio = proxyConfig.twcPortfolio;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected twcPortfolio: ${proxyConfig.twcPortfolio}`);
          }
          
          if (proxyConfig.twcGeography && !enhancedArgs.twcGeography) {
            enhancedArgs.twcGeography = proxyConfig.twcGeography;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected twcGeography: ${proxyConfig.twcGeography}`);
          }
          
          if (proxyConfig.filterCustomField && !enhancedArgs.filterCustomField) {
            enhancedArgs.filterCustomField = proxyConfig.filterCustomField;
            injectionsCount++;
            log.verbose(`[${requestId}] [${toolCallId}] Injected filterCustomField: ${proxyConfig.filterCustomField}`);
          }
          
          log.debug(`[${requestId}] [${toolCallId}] Injected ${injectionsCount} configuration parameters`);
          
          if (proxyConfig.verbose) {
            const safeArgs = { ...enhancedArgs, apiKey: enhancedArgs.apiKey ? '[REDACTED]' : 'Not set' };
            log.verbose(`[${requestId}] [${toolCallId}] Enhanced arguments:`, JSON.stringify(safeArgs, null, 2));
          }
          
          log.verbose(`[${requestId}] [${toolCallId}] Making tool execution request to remote server`);
          
          const response = await this.httpRequest(
            `${this.remoteUrl.origin}/tools/${name}/execute`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ arguments: enhancedArgs })
            }
          );
          
          log.verbose(`[${requestId}] [${toolCallId}] Received response from remote server`);
          
          const result = JSON.parse(response);
          const toolResponse = {
            jsonrpc: '2.0',
            id,
            result: {
              content: result.content || [{ type: 'text', text: response }],
              isError: result.isError || false
            }
          };
          
          const callDuration = Date.now() - startTime;
          log.info(`[${requestId}] [${toolCallId}] Tool execution completed in ${callDuration}ms`);
          
          if (proxyConfig.verbose) {
            log.verbose(`[${requestId}] [${toolCallId}] Tool response:`, JSON.stringify(toolResponse.result, null, 2));
          }
          
          return toolResponse;
          
        } catch (error) {
          const callDuration = Date.now() - startTime;
          log.error(`[${requestId}] [${toolCallId}] Tool execution failed after ${callDuration}ms: ${error.message}`);
          log.verbose(`[${requestId}] [${toolCallId}] Error stack:`, error.stack);
          
          const errorResponse = {
            jsonrpc: '2.0',
            id,
            result: {
              content: [{ type: 'text', text: `Error: ${error.message}` }],
              isError: true
            }
          };
          
          log.verbose(`[${requestId}] [${toolCallId}] Error response:`, JSON.stringify(errorResponse, null, 2));
          return errorResponse;
        }

      default:
        const unknownDuration = Date.now() - startTime;
        log.warn(`[${requestId}] Unknown method: ${method} (handled in ${unknownDuration}ms)`);
        
        const unknownResponse = {
          jsonrpc: '2.0',
          id,
          error: { code: -32601, message: 'Method not found' }
        };
        
        log.verbose(`[${requestId}] Unknown method response:`, JSON.stringify(unknownResponse, null, 2));
        return unknownResponse;
    }
  }

  start() {
    log.info(`MCP Proxy starting, connecting to: ${this.remoteUrl}`);
    log.verbose('Setting up stdin listener for MCP protocol messages');
    
    let messageCount = 0;
    
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', async (data) => {
      const lines = data.trim().split('\n');
      log.verbose(`Received ${lines.length} line(s) of input data`);
      
      for (const line of lines) {
        if (!line.trim()) continue;
        
        const msgId = `msg-${++messageCount}`;
        log.verbose(`[${msgId}] Processing message: ${line.substring(0, 100)}${line.length > 100 ? '...' : ''}`);
        
        try {
          const request = JSON.parse(line);
          log.verbose(`[${msgId}] Parsed JSON request successfully`);
          
          const response = await this.handleMCPRequest(request);
          if (response !== null) {
            log.verbose(`[${msgId}] Sending response to stdout`);
            console.log(JSON.stringify(response));
          } else {
            log.verbose(`[${msgId}] No response needed (notification)`);
          }
        } catch (error) {
          log.error(`[${msgId}] Error processing request: ${error.message}`);
          log.verbose(`[${msgId}] Error stack: ${error.stack}`);
          log.verbose(`[${msgId}] Raw input: ${line}`);
          
          // Try to extract ID from the line if possible
          let requestId = null;
          try {
            const parsedRequest = JSON.parse(line);
            requestId = parsedRequest.id || null;
            log.verbose(`[${msgId}] Extracted request ID: ${requestId}`);
          } catch (parseError) {
            log.verbose(`[${msgId}] Could not extract request ID: ${parseError.message}`);
          }
          
          const errorResponse = {
            jsonrpc: '2.0',
            id: requestId,
            error: { code: -32700, message: 'Parse error' }
          };
          
          log.verbose(`[${msgId}] Sending error response:`, JSON.stringify(errorResponse, null, 2));
          console.log(JSON.stringify(errorResponse));
        }
      }
    });

    process.stdin.on('end', () => {
      log.info('MCP Proxy received end signal, shutting down');
      log.verbose(`Final stats: ${this.requestCount} requests, ${this.toolCallCount} tool calls`);
      process.exit(0);
    });
    
    process.stdin.on('error', (error) => {
      log.error('Stdin error:', error.message);
      process.exit(1);
    });
    
    log.info('MCP Proxy ready to receive requests');
  }
}

const proxy = new MCPProxy(REMOTE_SSE_URL);
proxy.start();