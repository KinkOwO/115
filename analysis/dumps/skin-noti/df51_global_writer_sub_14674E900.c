// global_writer_sub_14674E900

__int64 __fastcall sub_14674E900(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rcx
  __int64 v4; // rdx
  volatile signed __int32 *v5; // rcx
  __int64 result; // rax
  __int64 v7; // [rsp+30h] [rbp-18h]

  dword_14DC6B63C = *(_DWORD *)a1;
  dword_14DC6B640 = *(_DWORD *)(a1 + 4);
  dword_14DC6B644 = *(_DWORD *)(a1 + 8);
  dword_14DC6B648 = *(_DWORD *)(a1 + 12);
  dword_14DC6B64C = *(_DWORD *)(a1 + 16);
  v2 = *(_QWORD *)(a1 + 32);
  v3 = 0;
  v4 = 0;
  if ( v2 )
  {
    v3 = *(_QWORD *)(a1 + 24);
    _InterlockedIncrement((volatile signed __int32 *)(v2 + 12));
    v4 = v2;
  }
  v7 = *((_QWORD *)&xmmword_14F1E5030 + 1);
  *(_QWORD *)&xmmword_14F1E5030 = v3;
  v5 = (volatile signed __int32 *)*((_QWORD *)&xmmword_14F1E5030 + 1);
  *((_QWORD *)&xmmword_14F1E5030 + 1) = v4;
  if ( v7 && _InterlockedExchangeAdd(v5 + 3, 0xFFFFFFFF) == 1 )
    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v5 + 8LL))(v5);
  dword_14DC6B650 = *(_DWORD *)(a1 + 40);
  dword_14DC6B654 = *(_DWORD *)(a1 + 44);
  dword_14DC6B658 = *(_DWORD *)(a1 + 48);
  dword_14DC6B65C = *(_DWORD *)(a1 + 52);
  dword_14DC6B660 = *(_DWORD *)(a1 + 56);
  dword_14DC6B664 = *(_DWORD *)(a1 + 60);
  dword_14DC6B668 = *(_DWORD *)(a1 + 64);
  dword_14DC6B66C = *(_DWORD *)(a1 + 68);
  qword_14F1E5020 = *(_QWORD *)(a1 + 72);
  dword_14DC6B670 = *(_DWORD *)(a1 + 80);
  dword_14DC6B674 = *(_DWORD *)(a1 + 84);
  dword_14DC6B678 = *(_DWORD *)(a1 + 88);
  dword_14DC6B67C = *(_DWORD *)(a1 + 92);
  dword_14DC6B680 = *(_DWORD *)(a1 + 96);
  dword_14DC6B684 = *(_DWORD *)(a1 + 100);
  dword_14DC6B688 = *(_DWORD *)(a1 + 104);
  result = *(unsigned int *)(a1 + 108);
  dword_14F1E5028 = *(_DWORD *)(a1 + 108);
  return result;
}

