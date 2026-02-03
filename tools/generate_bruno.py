#!/usr/bin/env python3
"""
Bruno API Collection Generator from OpenAPI/Swagger
Parses swagger.yaml and generates .bru request files organized by tags
"""

import yaml
import os
import re
from pathlib import Path
from typing import Dict, List, Any

def load_swagger(swagger_path: str) -> Dict:
    """Load and parse swagger.yaml"""
    with open(swagger_path, 'r') as f:
        return yaml.safe_load(f)

def sanitize_filename(name: str) -> str:
    """Convert endpoint name to valid filename"""
    # Remove special characters, replace spaces/slashes with hyphens
    name = re.sub(r'[^a-zA-Z0-9\s\-_]', '', name)
    name = re.sub(r'[\s/]+', '-', name)
    return name.strip('-').lower()

def get_method_color(method: str) -> str:
    """Get Bruno color for HTTP method"""
    colors = {
        'get': 'blue',
        'post': 'green',
        'put': 'orange',
        'patch': 'yellow',
        'delete': 'red',
        'head': 'purple',
        'options': 'gray'
    }
    return colors.get(method.lower(), 'white')

def extract_path_params(path: str) -> List[str]:
    """Extract parameter names from path like /stats/player/{guid}"""
    return re.findall(r'\{([^}]+)\}', path)

def generate_bru_content(
    method: str,
    path: str,
    tags: List[str],
    summary: str,
    description: str,
    parameters: List[Dict],
    security: List[Dict],
    request_body: Dict,
    responses: Dict
) -> str:
    """Generate .bru file content"""
    
    # Build URL with variable substitution
    url = path
    for param in extract_path_params(path):
        url = url.replace(f'{{{param}}}', f'{{{{var:{param}}}}}')
    
    # Start building .bru content
    bru = f'''meta {{
  name: {summary or path}
  type: http
  seq: 1
}}

{method.lower()} {{
  url: {{{{base_url}}}}{url}
  body: none
  auth: none
}}
'''

    # Add query parameters
    query_params = [p for p in parameters if p.get('in') == 'query']
    if query_params:
        bru += '\nparams:query {\n'
        for param in query_params:
            param_name = param.get('name', '')
            param_desc = param.get('description', '')
            default_val = param.get('default', '')
            required = '~' if not param.get('required', False) else ''
            
            if default_val:
                bru += f'  {required}{param_name}: {default_val}\n'
            else:
                bru += f'  {required}{param_name}: \n'
        bru += '}\n'

    # Add headers based on security requirements
    if security:
        bru += '\nheaders {\n'
        for sec in security:
            if 'ServerToken' in sec:
                bru += '  X-Server-Token: {{var:server_token}}\n'
            elif 'BearerAuth' in sec:
                bru += '  Authorization: Bearer {{var:bearer_token}}\n'
        bru += '}\n'

    # Add request body if POST/PUT/PATCH
    if request_body and method.lower() in ['post', 'put', 'patch']:
        content_type = list(request_body.get('content', {}).keys())[0] if request_body.get('content') else 'application/json'
        
        # Update body type
        bru = bru.replace('body: none', f'body: json')
        
        bru += f'\nbody:json {{\n'
        if 'application/json' in content_type:
            # Try to generate example JSON from schema
            schema = request_body.get('content', {}).get(content_type, {}).get('schema', {})
            bru += '  {\n'
            bru += '    // Add request body here\n'
            bru += '  }\n'
        bru += '}\n'

    # Add documentation
    if description or summary:
        bru += f'\ndocs {{\n'
        if summary:
            bru += f'  # {summary}\n'
            bru += f'  \n'
        if description and description != summary:
            bru += f'  {description}\n'
            bru += f'  \n'
        
        # Add response examples
        if responses:
            bru += f'  ## Responses\n'
            bru += f'  \n'
            for status_code, resp in responses.items():
                resp_desc = resp.get('description', status_code)
                bru += f'  - **{status_code}**: {resp_desc}\n'
        
        bru += '}\n'

    # Add tests/assertions
    bru += '''
tests {
  test("Status code is 2xx", function() {
    expect(res.status).to.be.within(200, 299);
  });
}
'''

    return bru

def create_bruno_collection(swagger_path: str, output_dir: str):
    """Generate complete Bruno collection from swagger spec"""
    
    swagger = load_swagger(swagger_path)
    base_path = swagger.get('basePath', '')
    paths = swagger.get('paths', {})
    
    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)
    
    # Track files created
    created_files = []
    folder_stats = {}
    
    # Process each endpoint
    for path, methods in paths.items():
        for method, spec in methods.items():
            if method.lower() not in ['get', 'post', 'put', 'patch', 'delete', 'head', 'options']:
                continue
            
            # Get metadata
            tags = spec.get('tags', ['Uncategorized'])
            tag = tags[0] if tags else 'Uncategorized'
            summary = spec.get('summary', path)
            description = spec.get('description', '')
            parameters = spec.get('parameters', [])
            security = spec.get('security', [])
            request_body = spec.get('requestBody', {})
            responses = spec.get('responses', {})
            
            # Create folder for tag
            folder_path = output_path / tag
            folder_path.mkdir(exist_ok=True)
            
            # Generate filename
            filename = f"{method.upper()} {sanitize_filename(summary or path)}.bru"
            file_path = folder_path / filename
            
            # Generate .bru content
            bru_content = generate_bru_content(
                method=method,
                path=path,
                tags=tags,
                summary=summary,
                description=description,
                parameters=parameters,
                security=security,
                request_body=request_body,
                responses=responses
            )
            
            # Write file
            with open(file_path, 'w') as f:
                f.write(bru_content)
            
            created_files.append(str(file_path.relative_to(output_path)))
            folder_stats[tag] = folder_stats.get(tag, 0) + 1
    
    # Print summary
    print(f"\n✓ Bruno Collection Generated Successfully!")
    print(f"\n📁 Created {len(created_files)} request files in {len(folder_stats)} folders:\n")
    
    for folder, count in sorted(folder_stats.items()):
        print(f"  • {folder}: {count} requests")
    
    print(f"\n📍 Collection root: {output_path}")
    print(f"\n💡 Next steps:")
    print(f"  1. Open Bruno and import collection from: {output_path}")
    print(f"  2. Select environment (Local/Development/Production)")
    print(f"  3. Update environment variables as needed")
    print(f"  4. Start testing! 🚀")

if __name__ == '__main__':
    import sys
    
    # Assume script is run from project root or tools dir
    script_dir = Path(__file__).parent
    project_root = script_dir.parent
    swagger_file = project_root / 'web' / 'static' / 'swagger.yaml'
    bruno_dir = project_root / 'bruno'
    
    if not swagger_file.exists():
        print(f"❌ Error: swagger.yaml not found at {swagger_file}")
        print(f"   Run 'make docs' to generate it first")
        sys.exit(1)
    
    print(f"🔄 Generating Bruno collection from {swagger_file.name}...")
    create_bruno_collection(str(swagger_file), str(bruno_dir))
