/**
 * AWS Lambda MCP Proxy
 * Converts MCP protocol to HTTP requests for serverless deployment
 */

const https = require('https');
const http = require('http');

// Configuration from environment variables
const SWAGGER_DOCS_URL = process.env.SWAGGER_DOCS_URL || 'https://your-ecs-alb.amazonaws.com';
const API_KEY = process.env.WX_MCP_API_KEY;

class LambdaMCPProxy {
  constructor() {
    this.tools = [];
    this.toolsLoaded = false;
  }

  async httpRequest(url, options = {}) {
    return new Promise((resolve, reject) => {
      const urlObj = new URL(url);
      const lib = urlObj.protocol === 'https:' ? https : http;
      
      const requestOptions = {
        hostname: urlObj.hostname,
        port: urlObj.port,
        path: urlObj.pathname + urlObj.search,
        method: options.method || 'GET',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'AWS-Lambda-MCP-Proxy/1.0',
          ...options.headers
        }
      };

      const req = lib.request(requestOptions, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            resolve(data);
          } else {
            reject(new Error(`HTTP ${res.statusCode}: ${data}`));
          }
        });
      });
      
      req.on('error', reject);
      req.setTimeout(25000, () => {
        req.abort();
        reject(new Error('Request timeout'));
      });

      if (options.body) {
        req.write(options.body);
      }
      req.end();
    });
  }

  async loadTools() {
    if (this.toolsLoaded) return;
    
    try {
      console.log('Loading tools from:', `${SWAGGER_DOCS_URL}/tools`);
      const response = await this.httpRequest(`${SWAGGER_DOCS_URL}/tools`);
      const data = JSON.parse(response);
      this.tools = data.tools || [];
      this.toolsLoaded = true;
      console.log(`Loaded ${this.tools.length} tools`);
    } catch (error) {
      console.error('Failed to load tools:', error.message);
      throw error;
    }
  }

  async handleMCPRequest(request) {
    const { method, params, id } = request;
    console.log('Handling MCP request:', method);

    try {
      switch (method) {
        case 'initialize':
          await this.loadTools();
          return {
            jsonrpc: '2.0',
            id,
            result: {
              protocolVersion: '2024-11-05',
              capabilities: {
                tools: { listChanged: true }
              },
              serverInfo: {
                name: 'swagger-docs-mcp-lambda',
                version: '1.0.0'
              }
            }
          };

        case 'tools/list':
          await this.loadTools();
          return {
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

        case 'tools/call':
          const { name, arguments: args } = params;
          console.log(`Executing tool: ${name}`);
          
          // Add API key to arguments if not present
          const toolArgs = { ...args };
          if (API_KEY && !toolArgs.apiKey) {
            toolArgs.apiKey = API_KEY;
          }

          const response = await this.httpRequest(
            `${SWAGGER_DOCS_URL}/tools/${name}/execute`,
            {
              method: 'POST',
              body: JSON.stringify({ arguments: toolArgs })
            }
          );
          
          const result = JSON.parse(response);
          return {
            jsonrpc: '2.0',
            id,
            result: {
              content: result.content || [{ type: 'text', text: response }],
              isError: result.isError || false
            }
          };

        default:
          return {
            jsonrpc: '2.0',
            id,
            error: { code: -32601, message: 'Method not found' }
          };
      }
    } catch (error) {
      console.error('Error handling MCP request:', error);
      return {
        jsonrpc: '2.0',
        id,
        error: { 
          code: -32603, 
          message: 'Internal error',
          data: error.message 
        }
      };
    }
  }
}

// Lambda handler for API Gateway
exports.handler = async (event, context) => {
  console.log('Lambda invoked with event:', JSON.stringify(event, null, 2));
  
  const proxy = new LambdaMCPProxy();
  
  try {
    // Handle different invocation types
    if (event.Records && event.Records[0].eventSource === 'aws:sqs') {
      // SQS-triggered batch processing
      const responses = [];
      for (const record of event.Records) {
        const request = JSON.parse(record.body);
        const response = await proxy.handleMCPRequest(request);
        responses.push(response);
      }
      return { batchItemFailures: [] };
    } else if (event.httpMethod) {
      // API Gateway HTTP request
      let request;
      try {
        request = JSON.parse(event.body || '{}');
      } catch (e) {
        return {
          statusCode: 400,
          headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
          },
          body: JSON.stringify({ error: 'Invalid JSON in request body' })
        };
      }

      const response = await proxy.handleMCPRequest(request);
      
      return {
        statusCode: 200,
        headers: {
          'Content-Type': 'application/json',
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Headers': 'Content-Type,Authorization',
          'Access-Control-Allow-Methods': 'GET,POST,OPTIONS'
        },
        body: JSON.stringify(response)
      };
    } else {
      // Direct Lambda invocation (for local MCP proxy)
      return await proxy.handleMCPRequest(event);
    }
  } catch (error) {
    console.error('Lambda handler error:', error);
    
    if (event.httpMethod) {
      return {
        statusCode: 500,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ error: 'Internal server error' })
      };
    } else {
      throw error;
    }
  }
};