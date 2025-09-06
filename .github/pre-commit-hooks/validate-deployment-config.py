#!/usr/bin/env python3
"""
Pre-commit hook to validate deployment configuration
Prevents deployment failures by validating configuration before commit
"""

import os
import sys
import json
import yaml
import re
from pathlib import Path
from typing import List, Dict, Any, Optional

class DeploymentConfigValidator:
    def __init__(self):
        self.errors = []
        self.warnings = []
        
    def validate_github_workflows(self) -> bool:
        """Validate GitHub workflow files"""
        workflows_dir = Path('.github/workflows')
        if not workflows_dir.exists():
            self.errors.append("No .github/workflows directory found")
            return False
            
        required_workflows = [
            'comprehensive-deployment.yml',
            'runner-optimization.yml', 
            'emergency-rollback.yml'
        ]
        
        for workflow in required_workflows:
            workflow_path = workflows_dir / workflow
            if not workflow_path.exists():
                self.errors.append(f"Missing required workflow: {workflow}")
                continue
                
            try:
                with open(workflow_path) as f:
                    workflow_content = yaml.safe_load(f)
                    self._validate_workflow_structure(workflow, workflow_content)
            except yaml.YAMLError as e:
                self.errors.append(f"Invalid YAML in {workflow}: {e}")
            except Exception as e:
                self.errors.append(f"Error reading {workflow}: {e}")
                
        return len(self.errors) == 0
        
    def _validate_workflow_structure(self, workflow_name: str, content: Dict[str, Any]):
        """Validate individual workflow structure"""
        if 'name' not in content:
            self.errors.append(f"{workflow_name}: Missing 'name' field")
            
        if 'on' not in content:
            self.errors.append(f"{workflow_name}: Missing 'on' triggers")
            
        if 'jobs' not in content:
            self.errors.append(f"{workflow_name}: Missing 'jobs' section")
            
        # Validate specific workflow requirements
        if workflow_name == 'comprehensive-deployment.yml':
            self._validate_deployment_workflow(content)
        elif workflow_name == 'emergency-rollback.yml':
            self._validate_rollback_workflow(content)
            
    def _validate_deployment_workflow(self, content: Dict[str, Any]):
        """Validate deployment workflow specific requirements"""
        jobs = content.get('jobs', {})
        
        required_jobs = [
            'detect-changes',
            'pre-flight-security', 
            'syntax-validation',
            'quality-gates',
            'security-scan',
            'docker-build',
            'integration-tests',
            'deploy-blue-green'
        ]
        
        for job in required_jobs:
            if job not in jobs:
                self.errors.append(f"Deployment workflow missing required job: {job}")
                
        # Check for quality thresholds
        env_vars = content.get('env', {})
        required_thresholds = [
            'MIN_GO_COVERAGE',
            'MIN_JS_COVERAGE', 
            'MAX_CRITICAL_VULNS',
            'MAX_BUILD_TIME'
        ]
        
        for threshold in required_thresholds:
            if threshold not in env_vars:
                self.warnings.append(f"Missing quality threshold: {threshold}")
                
    def _validate_rollback_workflow(self, content: Dict[str, Any]):
        """Validate rollback workflow requirements"""
        jobs = content.get('jobs', {})
        
        required_jobs = [
            'emergency-assessment',
            'execute-rollback',
            'verify-rollback'
        ]
        
        for job in required_jobs:
            if job not in jobs:
                self.errors.append(f"Rollback workflow missing required job: {job}")
                
    def validate_docker_configs(self) -> bool:
        """Validate Docker configuration files"""
        docker_files = [
            'Dockerfile',
            'docker-compose.single.yml',
            'pi-processor/Dockerfile'
        ]
        
        for docker_file in docker_files:
            if not os.path.exists(docker_file):
                if 'single' in docker_file:
                    self.warnings.append(f"Docker config not found: {docker_file}")
                else:
                    self.errors.append(f"Required Docker config missing: {docker_file}")
                continue
                    
            if docker_file.endswith('.yml'):
                self._validate_docker_compose(docker_file)
            else:
                self._validate_dockerfile(docker_file)
                
        return len(self.errors) == 0
        
    def _validate_docker_compose(self, file_path: str):
        """Validate docker-compose file"""
        try:
            with open(file_path) as f:
                compose_content = yaml.safe_load(f)
                
            # Check for required services
            services = compose_content.get('services', {})
            if not services:
                self.errors.append(f"{file_path}: No services defined")
                return
                
            # Validate service configurations
            for service_name, service_config in services.items():
                if 'image' not in service_config and 'build' not in service_config:
                    self.errors.append(f"{file_path}: Service {service_name} missing image or build")
                    
                # Check for health checks in production services
                if service_name in ['sermon-uploader', 'minio'] and 'healthcheck' not in service_config:
                    self.warnings.append(f"{file_path}: Service {service_name} missing healthcheck")
                    
        except yaml.YAMLError as e:
            self.errors.append(f"{file_path}: Invalid YAML - {e}")
        except Exception as e:
            self.errors.append(f"{file_path}: Error reading file - {e}")
            
    def _validate_dockerfile(self, file_path: str):
        """Validate Dockerfile"""
        try:
            with open(file_path) as f:
                dockerfile_content = f.read()
                
            # Check for required instructions
            required_instructions = ['FROM', 'WORKDIR', 'COPY', 'RUN']
            for instruction in required_instructions:
                if not re.search(rf'^{instruction}\s+', dockerfile_content, re.MULTILINE | re.IGNORECASE):
                    self.warnings.append(f"{file_path}: Missing {instruction} instruction")
                    
            # Check for security best practices
            if 'USER root' in dockerfile_content and 'USER ' not in dockerfile_content.split('USER root')[1]:
                self.warnings.append(f"{file_path}: Running as root user - consider using non-root user")
                
            # Check for health check
            if 'HEALTHCHECK' not in dockerfile_content:
                self.warnings.append(f"{file_path}: Missing HEALTHCHECK instruction")
                
        except Exception as e:
            self.errors.append(f"{file_path}: Error reading Dockerfile - {e}")
            
    def validate_environment_configs(self) -> bool:
        """Validate environment configuration files"""
        components = ['backend', 'frontend', 'pi-processor']
        
        for component in components:
            env_example = f"{component}/.env.example"
            if os.path.exists(env_example):
                self._validate_env_file(env_example)
            else:
                if component != 'frontend':  # Frontend .env is optional
                    self.warnings.append(f"Missing environment example file: {env_example}")
                    
        return True
        
    def _validate_env_file(self, file_path: str):
        """Validate .env file format"""
        try:
            with open(file_path) as f:
                lines = f.readlines()
                
            for line_num, line in enumerate(lines, 1):
                line = line.strip()
                if not line or line.startswith('#'):
                    continue
                    
                if '=' not in line:
                    self.errors.append(f"{file_path}:{line_num}: Invalid env var format: {line}")
                    continue
                    
                key, value = line.split('=', 1)
                
                # Check for common security issues
                if any(sensitive in key.upper() for sensitive in ['PASSWORD', 'SECRET', 'KEY', 'TOKEN']):
                    if not value.startswith('your_') and not value.startswith('${') and len(value) > 10:
                        self.warnings.append(f"{file_path}:{line_num}: Possible hardcoded secret in example file")
                        
        except Exception as e:
            self.errors.append(f"{file_path}: Error reading env file - {e}")
            
    def validate_secret_usage(self) -> bool:
        """Validate proper secret usage in workflows"""
        workflows_dir = Path('.github/workflows')
        
        for workflow_file in workflows_dir.glob('*.yml'):
            try:
                with open(workflow_file) as f:
                    content = f.read()
                    
                # Check for hardcoded secrets
                secret_patterns = [
                    r'password\s*[:=]\s*["\'][^"\']{8,}["\']',
                    r'token\s*[:=]\s*["\'][^"\']{20,}["\']',
                    r'key\s*[:=]\s*["\'][^"\']{10,}["\']',
                    r'discord\.com/api/webhooks/\d+/[a-zA-Z0-9_-]+'
                ]
                
                for pattern in secret_patterns:
                    matches = re.findall(pattern, content, re.IGNORECASE)
                    if matches:
                        self.errors.append(f"{workflow_file.name}: Found hardcoded secrets - use GitHub Secrets instead")
                        
                # Check for proper secret usage
                if '${{ secrets.' in content:
                    # Good - using GitHub Secrets
                    pass
                elif any(sensitive in content.upper() for sensitive in ['PASSWORD', 'SECRET_KEY', 'TOKEN']):
                    # Check if it's in a comment or example
                    if not re.search(r'#.*(?:PASSWORD|SECRET|TOKEN)', content, re.IGNORECASE):
                        self.warnings.append(f"{workflow_file.name}: Consider using GitHub Secrets for sensitive data")
                        
            except Exception as e:
                self.errors.append(f"{workflow_file.name}: Error validating secrets - {e}")
                
        return len(self.errors) == 0
        
    def validate_dependency_files(self) -> bool:
        """Validate dependency management files"""
        dependency_files = {
            'backend/go.mod': self._validate_go_mod,
            'frontend/package.json': self._validate_package_json,
            'pi-processor/requirements.txt': self._validate_requirements_txt,
            'pi-processor/pyproject.toml': self._validate_pyproject_toml
        }
        
        for file_path, validator in dependency_files.items():
            if os.path.exists(file_path):
                try:
                    validator(file_path)
                except Exception as e:
                    self.errors.append(f"{file_path}: Validation error - {e}")
                    
        return len(self.errors) == 0
        
    def _validate_go_mod(self, file_path: str):
        """Validate go.mod file"""
        with open(file_path) as f:
            content = f.read()
            
        if not re.search(r'^module\s+\S+', content, re.MULTILINE):
            self.errors.append(f"{file_path}: Missing module declaration")
            
        if not re.search(r'^go\s+\d+\.\d+', content, re.MULTILINE):
            self.errors.append(f"{file_path}: Missing Go version")
            
        # Check for minimum Go version
        go_version_match = re.search(r'^go\s+(\d+\.\d+)', content, re.MULTILINE)
        if go_version_match:
            version = float(go_version_match.group(1))
            if version < 1.21:
                self.warnings.append(f"{file_path}: Go version {version} may be outdated")
                
    def _validate_package_json(self, file_path: str):
        """Validate package.json file"""
        with open(file_path) as f:
            package_data = json.load(f)
            
        required_fields = ['name', 'version', 'scripts']
        for field in required_fields:
            if field not in package_data:
                self.errors.append(f"{file_path}: Missing required field: {field}")
                
        # Check for required scripts
        scripts = package_data.get('scripts', {})
        required_scripts = ['build', 'lint']
        for script in required_scripts:
            if script not in scripts:
                self.warnings.append(f"{file_path}: Missing recommended script: {script}")
                
    def _validate_requirements_txt(self, file_path: str):
        """Validate requirements.txt file"""
        with open(file_path) as f:
            lines = f.readlines()
            
        for line_num, line in enumerate(lines, 1):
            line = line.strip()
            if not line or line.startswith('#'):
                continue
                
            # Basic format check
            if not re.match(r'^[a-zA-Z0-9_-]+([><=!~]+[\d.]+)?$', line):
                if '/' not in line and 'git+' not in line:  # Allow git dependencies
                    self.warnings.append(f"{file_path}:{line_num}: Unusual dependency format: {line}")
                    
    def _validate_pyproject_toml(self, file_path: str):
        """Validate pyproject.toml file"""
        try:
            import tomli
            with open(file_path, 'rb') as f:
                pyproject_data = tomli.load(f)
                
            if 'project' not in pyproject_data and 'tool' not in pyproject_data:
                self.warnings.append(f"{file_path}: Missing project or tool configuration")
                
        except ImportError:
            self.warnings.append(f"{file_path}: Cannot validate - tomli not available")
        except Exception as e:
            self.errors.append(f"{file_path}: Invalid TOML format - {e}")
            
    def run_validation(self) -> bool:
        """Run all validation checks"""
        print("🔍 Validating deployment configuration...")
        
        validation_steps = [
            ("GitHub Workflows", self.validate_github_workflows),
            ("Docker Configurations", self.validate_docker_configs),
            ("Environment Configs", self.validate_environment_configs),
            ("Secret Usage", self.validate_secret_usage),
            ("Dependency Files", self.validate_dependency_files)
        ]
        
        all_passed = True
        for step_name, validator in validation_steps:
            print(f"  Checking {step_name}...")
            if not validator():
                all_passed = False
                
        return all_passed
        
    def print_results(self):
        """Print validation results"""
        if self.errors:
            print("\n❌ VALIDATION ERRORS:")
            for error in self.errors:
                print(f"   • {error}")
                
        if self.warnings:
            print("\n⚠️  VALIDATION WARNINGS:")
            for warning in self.warnings:
                print(f"   • {warning}")
                
        if not self.errors and not self.warnings:
            print("\n✅ All deployment configuration checks passed!")
        elif not self.errors:
            print(f"\n✅ Validation passed with {len(self.warnings)} warnings")
        else:
            print(f"\n❌ Validation failed with {len(self.errors)} errors and {len(self.warnings)} warnings")

def main():
    """Main entry point"""
    if len(sys.argv) > 1 and sys.argv[1] == '--help':
        print("""
Deployment Configuration Validator

This hook validates deployment configuration files to prevent failures:
- GitHub workflow files (.github/workflows/*.yml)
- Docker configurations (Dockerfile, docker-compose.yml)
- Environment configuration files (.env.example)
- Dependency management files (go.mod, package.json, requirements.txt)
- Secret usage patterns

Usage:
  python validate-deployment-config.py [--help]

Exit codes:
  0 - All validations passed
  1 - Validation errors found (will block commit)
  2 - Only warnings found (commit allowed)
""")
        return 0
        
    validator = DeploymentConfigValidator()
    
    try:
        validation_passed = validator.run_validation()
        validator.print_results()
        
        if not validation_passed:
            return 1
        elif validator.warnings:
            return 2
        else:
            return 0
            
    except KeyboardInterrupt:
        print("\n⏹️  Validation interrupted")
        return 1
    except Exception as e:
        print(f"\n💥 Validation failed with unexpected error: {e}")
        return 1

if __name__ == '__main__':
    sys.exit(main())