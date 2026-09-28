// global_writer_sub_146ED1F20

__int64 __fastcall sub_146ED1F20(__int64 a1)
{
  unsigned __int64 v2; // rax

  if ( *(_BYTE *)(a1 + 466) )
  {
    dword_14DC6B63C = -1;
    dword_14DC6B640 = -1;
    dword_14DC6B644 = -1;
    dword_14DC6B648 = -1;
    dword_14DC6B688 = -1;
    dword_14DC6B654 = -1;
    dword_14DC6B658 = -1;
  }
  if ( (unsigned __int8)sub_146EC45B0() )
  {
    v2 = sub_146EC45D0(*(_QWORD *)(a1 + 400));
    sub_146ED0BB0(a1, (unsigned int)v2, HIDWORD(v2));
  }
  return sub_146ECB560(a1);
}

