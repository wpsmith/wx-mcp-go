#!/usr/bin/env node

/**
 * AWS SSM Session Manager Tunnel Proxy
 * Creates a secure tunnel to ECS/EC2 instances via AWS Systems Manager
 */

const { spawn } = require('child_process');
const { promisify } = require('util');
const fs = require('fs').promises;
const path = require('path');

class SSMTunnelProxy {
  constructor(options = {}) {
    this.instanceId = options.instanceId || process.env.AWS_INSTANCE_ID;
    this.region = options.region || process.env.AWS_REGION || 'us-east-1';
    this.localPort = options.localPort || 8080;
    this.remotePort = options.remotePort || 8080;
    this.profile = options.profile || process.env.AWS_PROFILE;
    this.tunnelProcess = null;
    this.mcpProcess = null;
  }

  async startTunnel() {
    if (this.tunnelProcess) {
      console.error('Tunnel already running');
      return;
    }

    console.error(`Starting SSM tunnel: ${this.instanceId}:${this.remotePort} -> localhost:${this.localPort}`);

    const args = [
      'ssm', 'start-session',
      '--target', this.instanceId,
      '--document-name', 'AWS-StartPortForwardingSession',
      '--parameters', `portNumber=${this.remotePort},localPortNumber=${this.localPort}`,
      '--region', this.region
    ];

    if (this.profile) {
      args.push('--profile', this.profile);
    }

    this.tunnelProcess = spawn('aws', args, {
      stdio: ['pipe', 'pipe', 'inherit']
    });

    this.tunnelProcess.on('exit', (code, signal) => {
      console.error(`SSM tunnel exited with code ${code}, signal ${signal}`);
      this.tunnelProcess = null;
    });

    this.tunnelProcess.on('error', (error) => {
      console.error('SSM tunnel error:', error);
    });

    // Wait for tunnel to establish
    await new Promise(resolve => setTimeout(resolve, 3000));
    console.error('SSM tunnel established');
  }

  async stopTunnel() {
    if (this.tunnelProcess) {
      this.tunnelProcess.kill('SIGTERM');
      this.tunnelProcess = null;
      console.error('SSM tunnel stopped');
    }
  }

  async startMCPProxy() {
    console.error('Starting MCP proxy...');
    
    // Use the local MCP proxy to connect via tunnel
    const proxyPath = path.join(__dirname, '..', 'examples', 'mcp-proxy.js');
    
    this.mcpProcess = spawn('node', [proxyPath, `http://localhost:${this.localPort}`], {
      stdio: ['inherit', 'inherit', 'inherit']
    });

    this.mcpProcess.on('exit', (code, signal) => {
      console.error(`MCP proxy exited with code ${code}, signal ${signal}`);
      this.mcpProcess = null;
    });

    this.mcpProcess.on('error', (error) => {
      console.error('MCP proxy error:', error);
    });
  }

  async start() {
    try {
      await this.startTunnel();
      await this.startMCPProxy();
      
      // Handle cleanup on exit
      process.on('SIGINT', () => this.cleanup());
      process.on('SIGTERM', () => this.cleanup());
      process.on('exit', () => this.cleanup());
      
      console.error('SSM tunnel proxy ready');
    } catch (error) {
      console.error('Failed to start SSM tunnel proxy:', error);
      await this.cleanup();
      process.exit(1);
    }
  }

  async cleanup() {
    console.error('Cleaning up...');
    if (this.mcpProcess) {
      this.mcpProcess.kill('SIGTERM');
    }
    await this.stopTunnel();
  }
}

// Command line usage
if (require.main === module) {
  const instanceId = process.argv[2];
  const region = process.argv[3];
  
  if (!instanceId) {
    console.error('Usage: node ssm-tunnel-proxy.js <instance-id> [region]');
    console.error('Example: node ssm-tunnel-proxy.js i-1234567890abcdef0 us-east-1');
    process.exit(1);
  }

  const proxy = new SSMTunnelProxy({
    instanceId,
    region,
    localPort: 8080,
    remotePort: 8080
  });

  proxy.start().catch(error => {
    console.error('Failed to start proxy:', error);
    process.exit(1);
  });
}

module.exports = SSMTunnelProxy;