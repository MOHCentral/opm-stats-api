#!/usr/bin/env python3
"""
Comprehensive Event System Audit Script

Validates 100% consistency between:
- Go API (struct definitions, JSON tags)  
- Bruno API tests (JSON payloads)
- Event type enumerations
- GUID field naming conventions

Exit codes:
  0 = All checks passed
  1 = Validation failures found
"""

import json
import re
import sys
from pathlib import Path
from typing import Dict, List, Set, Tuple, Optional
from dataclasses import dataclass, field
from collections import defaultdict

# Color codes for terminal output
class Color:
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    CYAN = '\033[96m'
    BOLD = '\033[1m'
    END = '\033[0m'

@dataclass
class ValidationResult:
    """Tracks validation results"""
    passed: int = 0
    failed: int = 0
    warnings: int = 0
    errors: List[str] = field(default_factory=list)
    
    def add_error(self, msg: str):
        self.errors.append(msg)
        self.failed += 1
    
    def add_pass(self):
        self.passed += 1
    
    def add_warning(self, msg: str):
        self.errors.append(f"⚠️  {msg}")
        self.warnings += 1
    
    def is_success(self) -> bool:
        return self.failed == 0

@dataclass  
class GoStructField:
    """Represents a Go struct field with its JSON tag"""
    name: str
    go_type: str
    json_name: str
    is_required: bool  # True if no omitempty tag
    
@dataclass
class BrunoEvent:
    """Represents a parsed Bruno .bru file"""
    file_path: Path
    event_name: str
    event_type: str
    payload: Dict
    
class EventSystemAuditor:
    """Main auditor class"""
    
    def __init__(self, base_dir: Path):
        self.base_dir = base_dir
        self.go_fields: Dict[str, GoStructField] = {}
        self.bruno_events: List[BrunoEvent] = []
        self.event_types_go: Set[str] = set()
        self.event_types_bruno: Set[str] = set()
        self.results = ValidationResult()
        
    def run_audit(self) -> ValidationResult:
        """Execute full audit"""
        print(f"{Color.BOLD}{Color.CYAN}🔍 EVENT SYSTEM AUDIT{Color.END}\n")
        
        # Phase 1: Parse Go struct
        print(f"{Color.BLUE}[1/6] Parsing Go RawEvent struct...{Color.END}")
        self.parse_go_struct()
        print(f"      Found {len(self.go_fields)} fields in RawEvent struct")
        
        # Phase 2: Parse event types enum
        print(f"\n{Color.BLUE}[2/6] Parsing EventType enum...{Color.END}")
        self.parse_event_types_enum()
        print(f"      Found {len(self.event_types_go)} event types in Go")
        
        # Phase 3: Parse Bruno files
        print(f"\n{Color.BLUE}[3/6] Parsing Bruno .bru files...{Color.END}")
        self.parse_bruno_files()
        print(f"      Found {len(self.bruno_events)} Bruno event files")
        
        # Phase 4: Validate field names
        print(f"\n{Color.BLUE}[4/6] Validating Bruno payloads against Go struct...{Color.END}")
        self.validate_bruno_payloads()
        
        # Phase 5: Validate GUID fields
        print(f"\n{Color.BLUE}[5/6] Validating GUID field disambiguation...{Color.END}")
        self.validate_guid_fields()
        
        # Phase 6: Validate event counts
        print(f"\n{Color.BLUE}[6/6] Validating event type counts...{Color.END}")
        self.validate_event_counts()
        
        # Print summary
        self.print_summary()
        
        return self.results
    
    def parse_go_struct(self):
        """Parse internal/models/events.go for RawEvent struct and JSON tags"""
        events_file = self.base_dir / "internal" / "models" / "events.go"
        
        if not events_file.exists():
            self.results.add_error(f"Cannot find {events_file}")
            return
        
        content = events_file.read_text()
        
        # Extract RawEvent struct
        struct_match = re.search(
            r'type RawEvent struct \{(.*?)\n\}',
            content,
            re.DOTALL
        )
        
        if not struct_match:
            self.results.add_error("Cannot find RawEvent struct definition")
            return
        
        struct_body = struct_match.group(1)
        
        # Parse each field
        field_pattern = r'(\w+)\s+([\w\.\*\[\]]+)\s+`json:"([^"]+)"`'
        for match in re.finditer(field_pattern, struct_body):
            field_name = match.group(1)
            go_type = match.group(2)
            json_tag = match.group(3)
            
            # Parse JSON tag
            json_parts = json_tag.split(',')
            json_name = json_parts[0]
            is_required = 'omitempty' not in json_parts
            
            self.go_fields[json_name] = GoStructField(
                name=field_name,
                go_type=go_type,
                json_name=json_name,
                is_required=is_required
            )
    
    def parse_event_types_enum(self):
        """Parse event types from event_types_generated.go"""
        enum_file = self.base_dir / "internal" / "models" / "event_types_generated.go"
        
        if not enum_file.exists():
            self.results.add_error(f"Cannot find {enum_file}")
            return
        
        content = enum_file.read_text()
        
        # Extract all EventType constants
        const_pattern = r'Event\w+\s+EventType\s+=\s+"([^"]+)"'
        
        for match in re.finditer(const_pattern, content):
            event_value = match.group(1)
            self.event_types_go.add(event_value)
    
    def parse_bruno_files(self):
        """Parse all .bru files in bruno/Ingestion/Events/"""
        bruno_dir = self.base_dir / "bruno" / "Ingestion" / "Events"
        
        if not bruno_dir.exists():
            self.results.add_error(f"Cannot find Bruno events directory: {bruno_dir}")
            return
        
        for bru_file in sorted(bruno_dir.glob("*.bru")):
            try:
                event = self.parse_bru_file(bru_file)
                if event:
                    self.bruno_events.append(event)
                    self.event_types_bruno.add(event.event_type)
            except Exception as e:
                self.results.add_error(f"Failed to parse {bru_file.name}: {e}")
    
    def parse_bru_file(self, file_path: Path) -> Optional[BrunoEvent]:
        """Parse a single .bru file and extract JSON payload"""
        content = file_path.read_text()
        
        # Extract event name from meta block
        name_match = re.search(r'meta \{[^}]*name:\s*([^\n]+)', content)
        event_name = name_match.group(1).strip() if name_match else file_path.stem
        
        # Extract JSON payload from body:json block
        json_match = re.search(r'body:json \{(.*?)\n\}', content, re.DOTALL)
        
        if not json_match:
            return None
        
        json_text = json_match.group(1).strip()
        
        try:
            payload = json.loads(json_text)
            event_type = payload.get('type', '')
            
            return BrunoEvent(
                file_path=file_path,
                event_name=event_name,
                event_type=event_type,
                payload=payload
            )
        except json.JSONDecodeError as e:
            self.results.add_error(f"{file_path.name}: Invalid JSON - {e}")
            return None
    
    def validate_bruno_payloads(self):
        """Validate each Bruno payload against Go struct"""
        print()
        
        field_mismatches = []
        unknown_fields = []
        missing_type = []
        
        for event in self.bruno_events:
            # Check if 'type' field exists
            if 'type' not in event.payload:
                missing_type.append(event.file_path.name)
                self.results.add_error(
                    f"❌ {event.file_path.name}: Missing required 'type' field"
                )
                continue
            
            # Check each field in payload
            for field_name, field_value in event.payload.items():
                if field_name not in self.go_fields:
                    # Check for common mistakes
                    if field_name == 'event_type':
                        field_mismatches.append((event.file_path.name, field_name, 'type'))
                        self.results.add_error(
                            f"❌ {event.file_path.name}: Uses 'event_type' instead of 'type'"
                        )
                    elif field_name == 'guid':
                        unknown_fields.append((event.file_path.name, field_name))
                        self.results.add_error(
                            f"❌ {event.file_path.name}: Ambiguous 'guid' field (should be player_guid, attacker_guid, or victim_guid)"
                        )
                    else:
                        unknown_fields.append((event.file_path.name, field_name))
                        self.results.add_error(
                            f"❌ {event.file_path.name}: Unknown field '{field_name}' (not in Go struct)"
                        )
        
        # Summary for this phase
        if not field_mismatches and not unknown_fields and not missing_type:
            print(f"      {Color.GREEN}✓ All Bruno payloads match Go struct schema{Color.END}")
            self.results.add_pass()
        else:
            print(f"      {Color.RED}✗ Found {len(field_mismatches) + len(unknown_fields) + len(missing_type)} payload validation errors{Color.END}")
    
    def validate_guid_fields(self):
        """Validate GUID fields are properly disambiguated"""
        print()
        
        combat_events = {
            'player_kill', 'bot_killed', 'team_kill', 'damage', 
            'player_damage', 'bot_damage', 'team_damage'
        }
        
        single_player_events = {
            'jump', 'crouch', 'prone', 'stand', 'sprint', 'walk',
            'spawn', 'respawn', 'say', 'say_team', 'connect', 'disconnect',
            'weapon_pickup', 'weapon_switch', 'weapon_drop', 'reload',
            'item_pickup', 'item_use', 'item_drop'
        }
        
        ambiguous_guid_found = False
        
        for event in self.bruno_events:
            has_ambiguous_guid = 'guid' in event.payload
            has_player_guid = 'player_guid' in event.payload
            has_attacker_guid = 'attacker_guid' in event.payload
            has_victim_guid = 'victim_guid' in event.payload
            
            # Check for ambiguous 'guid' field
            if has_ambiguous_guid:
                ambiguous_guid_found = True
                self.results.add_error(
                    f"❌ {event.file_path.name}: Uses ambiguous 'guid' field"
                )
                continue
            
            # Validate combat events have correct GUID fields
            if event.event_type in combat_events:
                if not (has_attacker_guid and has_victim_guid):
                    expected = "attacker_guid AND victim_guid"
                    actual = []
                    if has_attacker_guid:
                        actual.append("attacker_guid")
                    if has_victim_guid:
                        actual.append("victim_guid")
                    if has_player_guid:
                        actual.append("player_guid")
                    
                    actual_str = ", ".join(actual) if actual else "none"
                    self.results.add_warning(
                        f"{event.file_path.name}: Combat event should have {expected}, has {actual_str}"
                    )
            
            # Validate single-player events have correct GUID field
            elif event.event_type in single_player_events:
                if not has_player_guid:
                    self.results.add_warning(
                        f"{event.file_path.name}: Single-player event should have player_guid"
                    )
        
        if not ambiguous_guid_found and self.results.warnings == 0:
            print(f"      {Color.GREEN}✓ All GUID fields properly disambiguated{Color.END}")
            self.results.add_pass()
        elif ambiguous_guid_found:
            print(f"      {Color.RED}✗ Found ambiguous 'guid' fields{Color.END}")
        else:
            print(f"      {Color.YELLOW}⚠ Found GUID field warnings{Color.END}")
    
    def validate_event_counts(self):
        """Validate event counts match across systems"""
        print()
        
        expected_count = 105
        go_count = len(self.event_types_go)
        bruno_count = len(self.event_types_bruno)
        bruno_file_count = len(self.bruno_events)
        
        all_match = (
            go_count == expected_count and
            bruno_count == expected_count and
            bruno_file_count == expected_count
        )
        
        if all_match:
            print(f"      {Color.GREEN}✓ Event counts match: {expected_count} events{Color.END}")
            self.results.add_pass()
        else:
            print(f"      {Color.RED}✗ Event count mismatch:{Color.END}")
            print(f"        Expected: {expected_count}")
            print(f"        Go enum: {go_count}")
            print(f"        Bruno event types: {bruno_count}")
            print(f"        Bruno files: {bruno_file_count}")
            
            self.results.add_error(
                f"Event count mismatch: Go={go_count}, Bruno Types={bruno_count}, Bruno Files={bruno_file_count}, Expected={expected_count}"
            )
        
        # Check for missing events
        missing_in_bruno = self.event_types_go - self.event_types_bruno
        extra_in_bruno = self.event_types_bruno - self.event_types_go
        
        if missing_in_bruno:
            print(f"\n      {Color.RED}Missing in Bruno:{Color.END}")
            for event in sorted(missing_in_bruno):
                print(f"        - {event}")
                self.results.add_error(f"Event type '{event}' in Go but missing in Bruno")
        
        if extra_in_bruno:
            print(f"\n      {Color.RED}Extra in Bruno (not in Go):{Color.END}")
            for event in sorted(extra_in_bruno):
                print(f"        - {event}")
                self.results.add_error(f"Event type '{event}' in Bruno but not in Go enum")
    
    def print_summary(self):
        """Print audit summary"""
        print(f"\n{Color.BOLD}{'='*60}{Color.END}")
        print(f"{Color.BOLD}AUDIT SUMMARY{Color.END}")
        print(f"{'='*60}")
        
        print(f"\n{Color.CYAN}Statistics:{Color.END}")
        print(f"  Go struct fields: {len(self.go_fields)}")
        print(f"  Event types (Go): {len(self.event_types_go)}")
        print(f"  Bruno files: {len(self.bruno_events)}")
        print(f"  Event types (Bruno): {len(self.event_types_bruno)}")
        
        print(f"\n{Color.CYAN}Results:{Color.END}")
        print(f"  {Color.GREEN}Passed: {self.results.passed}{Color.END}")
        print(f"  {Color.RED}Failed: {self.results.failed}{Color.END}")
        print(f"  {Color.YELLOW}Warnings: {self.results.warnings}{Color.END}")
        
        if self.results.errors:
            print(f"\n{Color.CYAN}Issues Found:{Color.END}")
            for i, error in enumerate(self.results.errors[:50], 1):  # Limit to first 50
                print(f"  {i}. {error}")
            
            if len(self.results.errors) > 50:
                print(f"  ... and {len(self.results.errors) - 50} more issues")
        
        print(f"\n{'='*60}")
        
        if self.results.is_success():
            print(f"{Color.GREEN}{Color.BOLD}✅ AUDIT PASSED{Color.END}")
            print(f"{Color.GREEN}All validation checks passed successfully!{Color.END}")
        else:
            print(f"{Color.RED}{Color.BOLD}❌ AUDIT FAILED{Color.END}")
            print(f"{Color.RED}Found {self.results.failed} validation errors that must be fixed.{Color.END}")
        
        print(f"{'='*60}\n")

def main():
    """Main entry point"""
    # Determine base directory (script is in tools/, we need parent)
    script_dir = Path(__file__).parent
    base_dir = script_dir.parent
    
    auditor = EventSystemAuditor(base_dir)
    results = auditor.run_audit()
    
    # Exit with appropriate code
    sys.exit(0 if results.is_success() else 1)

if __name__ == "__main__":
    main()
